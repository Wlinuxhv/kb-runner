package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const (
	// AdminToken 是开发者/管理员固定Token，可编辑SKILL
	AdminToken = "Admin@DEAD123"
	// CookieName Cookie名称
	CookieName = "kb_token"
	// CookieNameRole 角色Cookie名称
	CookieNameRole = "kb_role"
)

type Auth struct {
	token      string
	cookieName string
}

func NewAuth(token string) *Auth {
	if token == "" {
		return nil
	}
	return &Auth{
		token:      token,
		cookieName: CookieName,
	}
}

// IsAdminToken 检查是否为管理员Token
func IsAdminToken(token string) bool {
	return token == AdminToken
}

// GenerateRandomToken 生成随机Token (CTI/技服使用)
func GenerateRandomToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	if a == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// 公开路径无需认证
		if path == "/login" || path == "/api/v1/login" || path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		// 检查Cookie认证
		cookie, err := r.Cookie(a.cookieName)
		if err == nil && a.verifyToken(cookie.Value) {
			next.ServeHTTP(w, r)
			return
		}

		// 检查Header认证
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			if a.verifyToken(token) {
				a.setCookie(w, token)
				next.ServeHTTP(w, r)
				return
			}
		}

		// 未认证，重定向到登录页
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func (a *Auth) verifyToken(inputToken string) bool {
	// 直接验证明文开发者Token
	if inputToken == AdminToken {
		return true
	}
	// 直接验证明文配置Token
	if inputToken == a.token {
		return true
	}
	// 验证哈希过的Token（向后兼容）
	if inputToken == a.hashToken(AdminToken) {
		return true
	}
	if inputToken == a.hashToken(a.token) {
		return true
	}
	// 都不匹配
	return false
}

func (a *Auth) hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}

func (a *Auth) setCookie(w http.ResponseWriter, token string) {
	// 判断是否为管理员Token
	role := "user"
	if token == a.hashToken(AdminToken) {
		role = "admin"
	}

	// 设置认证Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	// 设置角色Cookie
	http.SetCookie(w, &http.Cookie{
		Name:    CookieNameRole,
		Value:   role,
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
	})
}

// GetRole 从请求中获取用户角色
func GetRole(r *http.Request) string {
	cookie, err := r.Cookie(CookieNameRole)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// IsAdmin 检查当前用户是否为管理员
func IsAdmin(r *http.Request) bool {
	return GetRole(r) == "admin"
}

func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>KB Runner - Login</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: #f8fafc;
        }
        .login-box {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            text-align: center;
            border: 1px solid #e2e8f0;
            width: 320px;
        }
        h1 {
            color: #2563eb;
            margin-bottom: 0.5rem;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .subtitle {
            color: #64748b;
            margin-bottom: 1.5rem;
            font-size: 0.875rem;
        }
        input {
            padding: 0.625rem 0.75rem;
            margin: 0.5rem 0;
            width: 100%;
            border: 1px solid #e2e8f0;
            border-radius: 4px;
            font-size: 0.875rem;
            outline: none;
        }
        input:focus {
            border-color: #2563eb;
        }
        button {
            padding: 0.625rem 1.5rem;
            background: #2563eb;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 0.875rem;
            cursor: pointer;
            margin-top: 1rem;
            width: 100%;
            transition: background 0.15s;
        }
        button:hover {
            background: #1d4ed8;
        }
        .error {
            color: #ef4444;
            margin-top: 0.75rem;
            font-size: 0.875rem;
        }
    </style>
</head>
<body>
    <div class="login-box">
        <h1>KB Runner</h1>
        <p class="subtitle">请输入访问Token</p>
        <form method="POST" action="/login">
            <input type="password" name="token" placeholder="Token" required>
            <button type="submit">登 录</button>
        </form>
    </div>
</body>
</html>`))
		return
	}

	r.ParseForm()
	inputToken := r.Form.Get("token")

	// 验证Token
	isValid := false
	isAdmin := false

	// 验证开发者Token (AdminDEFA123)
	if inputToken == AdminToken {
		isValid = true
		isAdmin = true
	} else if inputToken == a.token {
		// 验证配置的随机Token
		isValid = true
		isAdmin = false
	}

	if isValid {
		// 设置认证Cookie
		tokenHash := a.hashToken(inputToken)
		a.setCookie(w, tokenHash)

		// 如果是管理员，同步设置角色为admin
		if isAdmin {
			http.SetCookie(w, &http.Cookie{
				Name:    CookieNameRole,
				Value:   "admin",
				Path:    "/",
				Expires: time.Now().Add(24 * time.Hour),
			})
		}

		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>KB Runner - Login</title>
    <meta charset="utf-8">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: #f8fafc;
        }
        .login-box {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            text-align: center;
            border: 1px solid #e2e8f0;
            width: 320px;
        }
        h1 {
            color: #2563eb;
            margin-bottom: 0.5rem;
            font-size: 1.5rem;
            font-weight: 600;
        }
        .subtitle {
            color: #64748b;
            margin-bottom: 1.5rem;
            font-size: 0.875rem;
        }
        input {
            padding: 0.625rem 0.75rem;
            margin: 0.5rem 0;
            width: 100%;
            border: 1px solid #e2e8f0;
            border-radius: 4px;
            font-size: 0.875rem;
            outline: none;
        }
        input:focus {
            border-color: #2563eb;
        }
        button {
            padding: 0.625rem 1.5rem;
            background: #2563eb;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 0.875rem;
            cursor: pointer;
            margin-top: 1rem;
            width: 100%;
            transition: background 0.15s;
        }
        button:hover {
            background: #1d4ed8;
        }
        .error {
            color: #ef4444;
            margin-top: 0.75rem;
            font-size: 0.875rem;
        }
    </style>
</head>
<body>
    <div class="login-box">
        <h1>KB Runner</h1>
        <p class="subtitle">请输入访问Token</p>
        <form method="POST" action="/login">
            <input type="password" name="token" placeholder="Token" required>
            <button type="submit">登 录</button>
        </form>
        <p class="error">Token错误，请重试</p>
    </div>
</body>
</html>`))
	}
}
