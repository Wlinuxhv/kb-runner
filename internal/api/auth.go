package api

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

type Auth struct {
	token     string
	cookieName string
}

func NewAuth(token string) *Auth {
	if token == "" {
		return nil
	}
	return &Auth{
		token:     token,
		cookieName: "kb_token",
	}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	if a == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/login" || path == "/api/v1/login" || path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(a.cookieName)
		if err == nil && a.verifyToken(cookie.Value) {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			if a.verifyToken(token) {
				a.setCookie(w)
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func (a *Auth) verifyToken(token string) bool {
	expected := a.hashToken(a.token)
	return token == expected
}

func (a *Auth) hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}

func (a *Auth) setCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    a.hashToken(a.token),
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})
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
            font-family: 'Comic Sans MS', cursive, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
        }
        .login-box {
            background: white;
            padding: 50px 40px;
            border-radius: 20px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
            text-align: center;
            transform: rotate(-1deg);
            border: 3px solid #2D3436;
        }
        h1 {
            color: #FF6B6B;
            margin-bottom: 30px;
            font-size: 2rem;
        }
        .subtitle {
            color: #636E72;
            margin-bottom: 30px;
        }
        input {
            padding: 15px;
            margin: 10px 0;
            width: 280px;
            border: 2px solid #2D3436;
            border-radius: 15px 5px 15px 5px;
            font-size: 16px;
            outline: none;
        }
        input:focus {
            border-color: #FF6B6B;
        }
        button {
            padding: 15px 50px;
            background: #4ECDC4;
            color: white;
            border: 2px solid #2D3436;
            border-radius: 20px 5px 20px 5px;
            font-size: 18px;
            cursor: pointer;
            margin-top: 20px;
            font-family: 'Comic Sans MS', cursive;
            transition: transform 0.2s;
        }
        button:hover {
            transform: scale(1.05) rotate(1deg);
            background: #45B7AA;
        }
        .error {
            color: #FF6B6B;
            margin-top: 10px;
        }
    </style>
</head>
<body>
    <div class="login-box">
        <h1>🔐 KB Runner</h1>
        <p class="subtitle">请输入访问Token</p>
        <form method="POST" action="/login">
            <input type="password" name="token" placeholder="Token" required>
            <br>
            <button type="submit">登 录</button>
        </form>
    </div>
</body>
</html>`))
		return
	}

	r.ParseForm()
	inputToken := r.Form.Get("token")

	if a.verifyToken(inputToken) {
		a.setCookie(w)
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>KB Runner - Login</title>
    <meta charset="utf-8">
    <style>
        body { font-family: 'Comic Sans MS', cursive; display: flex; justify-content: center; align-items: center; min-height: 100vh; background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%); }
        .box { background: white; padding: 50px 40px; border-radius: 20px; box-shadow: 0 10px 30px rgba(0,0,0,0.1); text-align: center; transform: rotate(-1deg); border: 3px solid #2D3436; }
        h1 { color: #FF6B6B; margin-bottom: 20px; }
        input { padding: 15px; margin: 10px 0; width: 280px; border: 2px solid #2D3436; border-radius: 15px 5px 15px 5px; font-size: 16px; }
        button { padding: 15px 50px; background: #4ECDC4; color: white; border: 2px solid #2D3436; border-radius: 20px 5px 20px 5px; font-size: 18px; cursor: pointer; margin-top: 20px; font-family: 'Comic Sans MS', cursive; }
        button:hover { transform: scale(1.05); }
        .error { color: #FF6B6B; margin-top: 15px; }
    </style>
</head>
<body>
    <div class="box">
        <h1>🔐 KB Runner</h1>
        <p>请输入访问Token</p>
        <form method="POST" action="/login">
            <input type="password" name="token" placeholder="Token" required>
            <br>
            <button type="submit">登 录</button>
        </form>
        <p class="error">Token错误，请重试</p>
    </div>
</body>
</html>`))
	}
}
