const API_BASE = '/api/v1';
let currentKBs = [];
let currentScenarios = [];
let currentExecutions = [];
let filteredExecutions = [];
let selectedCases = new Set();
let currentResultId = null;
let currentUserRole = 'user';
let currentExecutionDetail = null;
let currentSkillCaseName = null;
let currentSkillContent = null;
let currentExecutionDir = null;
let currentExecutionData = null;
let deleteTarget = null;

async function apiRequest(url, options = {}) {
    const response = await fetch(url, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options.headers,
        },
    });
    const data = await response.json();
    if (!data.success) {
        throw new Error(data.error || 'Request failed');
    }
    return data.data;
}

async function loadKBs() {
    try {
        const cases = await apiRequest(`${API_BASE}/cases`);
        currentKBs = cases || [];
        renderKBs(currentKBs);
        updateCategoryFilter();
    } catch (error) {
        console.error('Failed to load cases:', error);
        document.getElementById('kbList').innerHTML = `
            <div class="empty-state">
                <h3>暂无KB</h3>
                <p>请先创建KB或检查配置</p>
            </div>`;
    }
}

function renderKBs(cases) {
    const container = document.getElementById('kbList');
    if (!cases || cases.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <h3>暂无KB</h3>
                <p>点击"执行选中"按钮开始执行</p>
            </div>`;
        return;
    }

    container.innerHTML = cases.map(c => `
        <div class="card">
            <div class="checkbox-wrapper">
                <input type="checkbox" data-name="${c.name}" onchange="toggleCaseSelect('${c.name}')">
            </div>
            <div class="card-header">
                <span class="card-title">${c.name}</span>
                <span class="card-category">${c.category || 'default'}</span>
            </div>
            <div class="card-description">${c.description || '无描述'}</div>
            <div class="card-tags">
                ${(c.tags || []).map(t => `<span class="tag">${t}</span>`).join('')}
            </div>
            <div class="card-actions">
                <button class="btn" onclick="event.preventDefault(); window.open('/skill-view.html?case=${encodeURIComponent(c.name)}', '_blank')">详情</button>
                <button class="btn btn-primary" onclick="executeSingleCase('${c.name}')">执行</button>
            </div>
        </div>
    `).join('');
}

function updateCategoryFilter() {
    const categories = new Set(currentKBs.map(c => c.category || 'default'));
    const select = document.getElementById('categoryFilter');
    const currentValue = select.value;
    select.innerHTML = '<option value="">全部分类</option>' +
        Array.from(categories).map(c => `<option value="${c}">${c}</option>`).join('');
    select.value = currentValue;
}

function filterKBs() {
    const category = document.getElementById('categoryFilter').value;
    const search = document.getElementById('searchInput').value.toLowerCase();

    const filtered = currentKBs.filter(c => {
        const matchCategory = !category || c.category === category;
        const matchSearch = !search ||
            c.name.toLowerCase().includes(search) ||
            (c.description || '').toLowerCase().includes(search);
        return matchCategory && matchSearch;
    });

    renderKBs(filtered);
}

function toggleCaseSelect(name) {
    if (selectedCases.has(name)) {
        selectedCases.delete(name);
    } else {
        selectedCases.add(name);
    }
}

async function loadScenarios() {
    try {
        const scenarios = await apiRequest(`${API_BASE}/scenarios`);
        currentScenarios = scenarios || [];
        renderScenarios(currentScenarios);
    } catch (error) {
        console.error('Failed to load scenarios:', error);
        document.getElementById('scenarioList').innerHTML = `
            <div class="empty-state">
                <h3>暂无场景</h3>
            </div>`;
    }
}

function renderScenarios(scenarios) {
    const container = document.getElementById('scenarioList');
    if (!scenarios || scenarios.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <h3>暂无场景</h3>
                <p>请在配置文件中定义场景</p>
            </div>`;
        return;
    }

    container.innerHTML = scenarios.map(s => `
        <div class="card">
            <div class="card-header">
                <span class="card-title">${s.name}</span>
            </div>
            <div class="card-description">${s.description || '无描述'}</div>
            <div class="card-tags">
                <span class="tag">${(s.cases || []).length} 个KB</span>
            </div>
            <div class="card-actions">
                <button class="btn btn-primary" onclick="executeScenario('${s.name}')">执行场景</button>
            </div>
        </div>
    `).join('');
}

async function loadHistory() {
    try {
        const executions = await apiRequest(`${API_BASE}/executions`);
        currentExecutions = executions || [];
        filteredExecutions = [...currentExecutions];
        renderExecutions(filteredExecutions);
        updateQnoFilter();
    } catch (error) {
        console.error('Failed to load executions:', error);
        document.getElementById('executionList').innerHTML = `
            <div class="empty-state">
                <h3>暂无执行记录</h3>
            </div>`;
    }
}

function renderExecutions(executions) {
    const container = document.getElementById('executionList');
    if (!executions || executions.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <h3>暂无执行记录</h3>
                <p>请先执行 KB 脚本</p>
            </div>`;
        return;
    }

    container.innerHTML = executions.map(e => `
        <div class="history-item" onclick="showExecutionDetail('${e.dir_name}')">
            <div class="history-info">
                <span class="history-qno">${e.qno}</span>
                <span class="history-id">${e.dir_name}</span>
                <span class="history-time">${formatTime(e.exec_time)}</span>
                <span class="tag">${e.script_count} 个 KB</span>
            </div>
            <div class="history-summary">
                <span>加权平均：${(e.summary?.weighted_average || 0).toFixed(2)}</span>
                <button class="btn btn-sm" onclick="event.stopPropagation(); showDeleteExecution('${e.dir_name}')">删除</button>
            </div>
        </div>
    `).join('');
}

function formatTime(timestamp) {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    return date.toLocaleString('zh-CN');
}

async function executeSelected() {
    if (selectedCases.size === 0) {
        alert('请先选择要执行的KB');
        return;
    }

    const names = Array.from(selectedCases);
    await executeCases(names);
}

async function executeSingleCase(name) {
    await executeCases([name]);
}

async function executeScenario(name) {
    try {
        showLoading();
        const result = await apiRequest(`${API_BASE}/execute`, {
            method: 'POST',
            body: JSON.stringify({
                type: 'scenario',
                name: name
            })
        });
        showResult(result);
        loadHistory();
    } catch (error) {
        alert('执行失败: ' + error.message);
    }
}

async function executeCases(names) {
    try {
        showLoading();
        const result = await apiRequest(`${API_BASE}/execute`, {
            method: 'POST',
            body: JSON.stringify({
                type: 'case',
                names: names
            })
        });
        showResult(result);
        loadHistory();
        selectedCases.clear();
        filterKBs();
    } catch (error) {
        alert('执行失败: ' + error.message);
    }
}

function showLoading() {
    document.getElementById('resultContent').innerHTML = '<div class="loading">正在执行</div>';
    document.getElementById('resultModal').classList.add('active');
}

function getScoreColorClass(score) {
    if (score >= 80) return 'chart-high';
    if (score >= 40) return 'chart-medium';
    return 'chart-low';
}

function renderYAxisLabels() {
    return [100, 80, 60, 40, 20, 0].map(val => 
        `<div class="chart-y-label" style="top: ${100 - val}%">${val}</div>`
    ).join('');
}

function renderBarChart(scripts) {
    if (!scripts || scripts.length === 0) return '';

    const hasNormalized = scripts.some(s => s.normalized_score !== undefined);
    const getValue = (s) => hasNormalized ? ((s.normalized_score ?? 0) * 100) : (s.score || 0);

    const sorted = [...scripts].sort((a, b) => getValue(b) - getValue(a));

    const maxPerRow = 12;
    const rows = [];
    for (let i = 0; i < sorted.length; i += maxPerRow) {
        rows.push(sorted.slice(i, i + maxPerRow));
    }

    let html = '';
    rows.forEach((rowScripts, rowIndex) => {
        let bars = rowScripts.map((s) => {
            const value = getValue(s);
            const height = Math.max((value / 100) * 100, 2);
            const colorClass = getScoreColorClass(value);
            const kbCode = s.script_name || s.name || 'Unknown';

            return `
                <div class="chart-bar-wrapper">
                    <div class="chart-bar ${colorClass}"
                         style="height: ${height}%"
                         onclick="showScriptDetail('${kbCode}')"
                         title="${kbCode}: ${value.toFixed(1)}分">
                        <span class="chart-score">${value.toFixed(0)}</span>
                    </div>
                    <div class="chart-label" title="${kbCode}">${kbCode.length > 12 ? kbCode.substring(0, 12) + '...' : kbCode}</div>
                </div>
            `;
        }).join('');

        html += `
            <div class="chart-container">
                <h3 class="chart-title">KB 得分排行</h3>
                <div class="chart-wrapper">
                    <div class="chart-y-axis">${renderYAxisLabels()}</div>
                    <div class="chart-bars" style="grid-template-columns: repeat(${rowScripts.length}, minmax(50px, 1fr));">${bars}</div>
                </div>
            </div>
        `;
    });

    html += '<div class="chart-tip">点击柱子查看 KB 执行详情</div>';
    return html;
}

function showResult(result) {
    currentResultId = result.execution_id;
    const summary = result.summary || {};
    const scripts = result.scripts || [];

    const barChartHtml = renderBarChart(scripts);

    let html = `
        ${barChartHtml}
        <div class="result-summary">
            <div class="stat-box">
                <div class="stat-value">${summary.total_scripts || 0}</div>
                <div class="stat-label">总数</div>
            </div>
            <div class="stat-box success">
                <div class="stat-value">${summary.success_count || 0}</div>
                <div class="stat-label">成功</div>
            </div>
            <div class="stat-box failure">
                <div class="stat-value">${summary.failure_count || 0}</div>
                <div class="stat-label">失败</div>
            </div>
            <div class="stat-box">
                <div class="stat-value">${(summary.weighted_average || 0).toFixed(2)}</div>
                <div class="stat-label">加权平均分</div>
            </div>
        </div>
        <table class="result-table">
            <thead>
                <tr>
                    <th>脚本</th>
                    <th>状态</th>
                    <th>原始得分</th>
                    <th>归一化得分</th>
                    <th>权重</th>
                    <th>耗时</th>
                    <th>操作</th>
                </tr>
            </thead>
            <tbody>
                ${scripts.map(s => `
                    <tr>
                        <td>${s.script_name || s.name}</td>
                        <td><span class="status-badge status-${s.status}">${s.status}</span></td>
                        <td>${(s.score || 0).toFixed(2)}</td>
                        <td>${(s.normalized_score || s.score || 0).toFixed(2)}</td>
                        <td>${s.weight || 1}</td>
                        <td>${s.duration_ms || 0}ms</td>
                        <td><button class="btn btn-sm" onclick="showScriptDetail('${s.script_name || s.name}')">详情</button></td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('resultContent').innerHTML = html;
    document.getElementById('resultModal').classList.add('active');
}

async function showScriptDetail(scriptName) {
    if (!currentExecutionDir) {
        alert('请先执行 KB');
        return;
    }

    try {
        const result = await apiRequest(`${API_BASE}/executions/${currentExecutionDir}`);
        const script = (result.scripts || []).find(s => s.name === scriptName);

        if (!script) {
            alert('未找到脚本详情');
            return;
        }

        currentExecutionDetail = {
            result: result,
            script: script,
            caseName: scriptName
        };

        renderScriptDetail(currentExecutionDetail.caseName, script, result);
        document.getElementById('detailModal').classList.add('active');
    } catch (error) {
        alert('获取详情失败：' + error.message);
    }
}
function renderScriptDetail(caseName, script, result) {
    const container = document.getElementById('detailContent');

    const scoreValue = ((script.normalized_score || 0) * 100).toFixed(0);
    const scoreLevel = scoreValue >= 80 ? 'high' : (scoreValue >= 40 ? 'medium' : 'low');

    const steps = script.steps || [];
    const stepsHtml = steps.length > 0 ? `
        <div class="detail-section">
            <h3>步骤详情</h3>
            <table class="result-table">
                <thead>
                    <tr>
                        <th>步骤</th>
                        <th>状态</th>
                        <th>匹配规则</th>
                        <th>输出</th>
                        <th>耗时</th>
                    </tr>
                </thead>
                <tbody>
                    ${steps.map(step => `
                        <tr>
                            <td>${step.name || '-'}</td>
                            <td><span class="status-badge status-${step.status}">${step.status}</span></td>
                            <td>${step.matched_rules || '-'}</td>
                            <td>${step.output || '-'}</td>
                            <td>${step.duration_ms || 0}ms</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    ` : '';

    // 获取Skill内容（从API获取）
    let skillHtml = '<div class="detail-loading">加载中...</div>';
    loadSkillContent(caseName).then(content => {
        skillHtml = `
            <div class="detail-section">
                <h3>Skill.md 内容</h3>
                <pre class="skill-content">${escapeHtml(content)}</pre>
            </div>
        `;
        // 更新skill部分
        const skillSection = container.querySelector('.skill-section-placeholder');
        if (skillSection) {
            skillSection.outerHTML = skillHtml;
        }
    }).catch(() => {
        skillHtml = '<div class="detail-section"><h3>Skill.md 内容</h3><p>无法加载Skill内容</p></div>';
    });

    // KB配置信息
    const kbConfig = `
        <div class="detail-section">
            <h3>KB配置</h3>
            <div class="config-grid">
                <div class="config-item">
                    <span class="config-label">KB编码:</span>
                    <span class="config-value">${caseName}</span>
                </div>
                <div class="config-item">
                    <span class="config-label">权重:</span>
                    <span class="config-value">${script.weight || 1}</span>
                </div>
                <div class="config-item">
                    <span class="config-label">执行状态:</span>
                    <span class="config-value"><span class="status-badge status-${script.status}">${script.status}</span></span>
                </div>
                <div class="config-item">
                    <span class="config-label">归一化得分:</span>
                    <span class="config-value">${(script.normalized_score || 0).toFixed(2)}</span>
                </div>
                <div class="config-item">
                    <span class="config-label">执行耗时:</span>
                    <span class="config-value">${script.duration_ms || 0}ms</span>
                </div>
            </div>
        </div>
    `;

    const logs = script.logs || script.output || '无日志';
    const logsHtml = `
        <div class="detail-section">
            <h3>执行日志</h3>
            <div class="log-actions">
                <button class="btn btn-sm" onclick="copyLogContent()">复制日志</button>
            </div>
            <pre class="log-content" id="logContent">${escapeHtml(logs)}</pre>
        </div>
    `;

    const editSkillBtn = currentUserRole === 'admin' ?
        `<button class="btn btn-primary" onclick="openSkillEditor('${caseName}')">编辑 Skill</button>` : '';

    container.innerHTML = `
        <div class="detail-header">
            <h3>${caseName}</h3>
            <div class="score-badge score-${scoreLevel}">${scoreValue}分</div>
            ${editSkillBtn}
        </div>
        ${kbConfig}
        <div class="skill-section-placeholder">${skillHtml}</div>
        ${stepsHtml}
        ${logsHtml}
    `;
}

function copyLogContent() {
    const logContent = document.getElementById('logContent');
    if (logContent) {
        navigator.clipboard.writeText(logContent.textContent).then(() => {
            alert('日志已复制到剪贴板');
        }).catch(() => {
            alert('复制失败');
        });
    }
}

async function loadSkillContent(caseName) {
    try {
        const data = await apiRequest(`${API_BASE}/kb/${caseName}/skill`);
        return data.content || '无内容';
    } catch (error) {
        return '无法加载Skill: ' + error.message;
    }
}

function escapeHtml(text) {
    if (!text) return '';
    return text.replace(/&/g, '&amp;')
               .replace(/</g, '&lt;')
               .replace(/>/g, '&gt;')
               .replace(/"/g, '&quot;')
               .replace(/'/g, '&#039;');
}

function closeDetailModal() {
    document.getElementById('detailModal').classList.remove('active');
    currentExecutionDetail = null;
}

// Skill编辑功能
async function openSkillEditor(caseName) {
    currentSkillCaseName = caseName;
    currentSkillContent = null;

    try {
        const data = await apiRequest(`${API_BASE}/kb/${caseName}/skill`);
        currentSkillContent = data.content || '';

        renderSkillEditor(caseName, currentSkillContent);
        document.getElementById('skillModal').classList.add('active');
    } catch (error) {
        alert('加载Skill失败: ' + error.message);
    }
}

function renderSkillEditor(caseName, content) {
    const container = document.getElementById('skillContent');
    container.innerHTML = `
        <div class="detail-section">
            <h3>编辑 Skill.md - ${caseName}</h3>
            <textarea id="skillEditor" class="skill-editor">${escapeHtml(content)}</textarea>
        </div>
    `;
}

async function saveSkill() {
    if (!currentSkillCaseName) return;

    const content = document.getElementById('skillEditor').value;

    try {
        await apiRequest(`${API_BASE}/kb/${currentSkillCaseName}/skill`, {
            method: 'PUT',
            body: JSON.stringify({ content: content })
        });
        alert('保存成功');
        closeSkillModal();
    } catch (error) {
        alert('保存失败: ' + error.message);
    }
}

function showSkillHistory() {
    if (!currentSkillCaseName) return;

    window.location.href = `/#/kb/${currentSkillCaseName}/skill/history`;
}

function closeSkillModal() {
    document.getElementById('skillModal').classList.remove('active');
    currentSkillCaseName = null;
    currentSkillContent = null;
}

async function showHistoryDetail(id) {
    try {
        const record = await apiRequest(`${API_BASE}/history/${id}`);
        showResult(record.result);
    } catch (error) {
        alert('获取历史详情失败: ' + error.message);
    }
}

function closeModal() {
    document.getElementById('resultModal').classList.remove('active');
    currentResultId = null;
}

document.getElementById('resultModal').addEventListener('click', (e) => {
    if (e.target.id === 'resultModal') {
        closeModal();
    }
});

document.getElementById('detailModal').addEventListener('click', (e) => {
    if (e.target.id === 'detailModal') {
        closeDetailModal();
    }
});

document.getElementById('skillModal').addEventListener('click', (e) => {
    if (e.target.id === 'skillModal') {
        closeSkillModal();
    }
});

async function exportResult(format) {
    if (!currentResultId) return;
    window.location.href = `${API_BASE}/history/${currentResultId}/export?format=${format}`;
}

async function showCaseDetail(name) {
    try {
        const c = await apiRequest(`${API_BASE}/cases/${name}`);
        alert(`KB: ${c.name}\n分类: ${c.category}\n描述: ${c.description}\n语言: ${c.language}\n超时: ${c.timeout}s\n权重: ${c.weight}`);
    } catch (error) {
        alert('获取详情失败: ' + error.message);
    }
}

function showPage(page) {
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('.nav-link').forEach(l => l.classList.remove('active'));

    document.getElementById(`page-${page}`).classList.add('active');
    document.querySelector(`[data-page="${page}"]`).classList.add('active');

    if (page === 'kb') {
        loadKBs();
    } else if (page === 'scenarios') {
        loadScenarios();
    } else if (page === 'history') {
        loadHistory();
    }
}

document.addEventListener('DOMContentLoaded', async () => {
    await loadUserRole();
    loadKBs();
});

async function loadUserRole() {
    try {
        const data = await apiRequest(`${API_BASE}/user/role`);
        currentUserRole = data.role || 'user';
    } catch (error) {
        console.error('Failed to load user role:', error);
        currentUserRole = 'user';
    }
}
async function showExecutionDetail(dirName) {
    try {
        const result = await apiRequest(`${API_BASE}/executions/${dirName}`);
        currentExecutionDir = dirName;
        currentExecutionData = result;
        
        const barChartHtml = renderBarChart(result.scripts || []);
        
        document.getElementById('detailTitle').textContent = `${result.qno || dirName} - ${result.execution_id}`;
        
        const summary = result.summary || {};
        document.getElementById('detailContent').innerHTML = `
            ${barChartHtml}
            <div class="result-summary">
                <div class="stat-box">
                    <div class="stat-value">${summary.total_scripts || 0}</div>
                    <div class="stat-label">总数</div>
                </div>
                <div class="stat-box success">
                    <div class="stat-value">${summary.success_count || 0}</div>
                    <div class="stat-label">成功</div>
                </div>
                <div class="stat-box failure">
                    <div class="stat-value">${summary.failure_count || 0}</div>
                    <div class="stat-label">失败</div>
                </div>
                <div class="stat-box">
                    <div class="stat-value">${(summary.weighted_average || 0).toFixed(2)}</div>
                    <div class="stat-label">加权平均分</div>
                </div>
            </div>
            <table class="result-table">
                <thead>
                    <tr>
                        <th>脚本</th>
                        <th>状态</th>
                        <th>原始得分</th>
                        <th>归一化得分</th>
                        <th>权重</th>
                        <th>耗时</th>
                    </tr>
                </thead>
                <tbody>
                    ${result.scripts.map(s => `
                        <tr>
                            <td>${s.name}</td>
                            <td><span class="status-badge status-${s.status}">${s.status}</span></td>
                            <td>${(s.raw_score || 0).toFixed(2)}</td>
                            <td>${((s.normalized_score || 0) * 100).toFixed(0)}%</td>
                            <td>${s.weight || 1}</td>
                            <td>${s.duration_ms || 0}ms</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
        
        document.getElementById('executionList').style.display = 'none';
        document.getElementById('executionDetail').style.display = 'block';
    } catch (error) {
        alert('获取详情失败：' + error.message);
    }
}

function backToList() {
    document.getElementById('executionList').style.display = 'block';
    document.getElementById('executionDetail').style.display = 'none';
    currentExecutionDir = null;
    currentExecutionData = null;
}

function showDeleteExecution(dirName) {
    deleteTarget = { type: 'execution', dirName };
    showDeleteModal(`确定要删除执行记录 ${dirName} 吗？`);
}

function showDeleteExecutionFromDetail() {
    if (currentExecutionDir) {
        showDeleteExecution(currentExecutionDir);
    }
}

function showDeleteQNo(qno) {
    deleteTarget = { type: 'qno', qno };
    showDeleteModal(`确定要删除 Q 单 ${qno} 的所有执行记录吗？`);
}

function confirmClearAll() {
    if (!confirm('确定要清除所有执行记录吗？此操作不可恢复！\n\n将在弹窗中输入 DELETE 确认。')) {
        return;
    }
    deleteTarget = { type: 'all' };
    showDeleteModal('确定要清除所有执行记录吗？此操作不可恢复！');
}

function showDeleteModal(message) {
    document.getElementById('deleteMessage').textContent = message;
    document.getElementById('deleteConfirm').value = '';
    document.getElementById('deleteModal').classList.add('active');
    document.getElementById('deleteConfirm').focus();
}

function closeDeleteModal() {
    document.getElementById('deleteModal').classList.remove('active');
    deleteTarget = null;
}

async function confirmDelete() {
    const confirmText = document.getElementById('deleteConfirm').value;
    if (confirmText !== 'DELETE') {
        alert('请输入 DELETE 确认删除');
        return;
    }
    
    try {
        let url;
        if (deleteTarget.type === 'execution') {
            url = `${API_BASE}/executions/${deleteTarget.dirName}`;
        } else if (deleteTarget.type === 'qno') {
            url = `${API_BASE}/qnos/${deleteTarget.qno}`;
        } else if (deleteTarget.type === 'all') {
            url = `${API_BASE}/executions`;
        }
        
        await apiRequest(url, { method: 'DELETE' });
        alert('删除成功');
        closeDeleteModal();
        loadHistory();
        backToList();
    } catch (error) {
        alert('删除失败：' + error.message);
    }
}

function updateQnoFilter() {
    const qnos = [...new Set(currentExecutions.map(e => e.qno))];
    const select = document.getElementById('qnoFilter');
    const currentValue = select.value;
    select.innerHTML = '<option value="">全部 Q 单</option>' +
        qnos.map(q => `<option value="${q}">${q}</option>`).join('');
    select.value = currentValue;
}

function filterExecutions() {
    const qno = document.getElementById('qnoFilter').value;
    filteredExecutions = qno 
        ? currentExecutions.filter(e => e.qno === qno)
        : [...currentExecutions];
    renderExecutions(filteredExecutions);
}
