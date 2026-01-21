// INFO.FILTER - 赛博朋克信息过滤器

class InfoFilter {
    constructor() {
        this.currentSource = '';
        this.init();
    }

    async init() {
        this.showCurrentDate();
        await this.loadStats();
        await this.loadItems();
        this.bindEvents();
    }

    showCurrentDate() {
        const now = new Date();
        const dateStr = now.toLocaleDateString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            weekday: 'short'
        });
        const dateEl = document.getElementById('current-date');
        if (dateEl) {
            dateEl.textContent = dateStr;
        }
    }

    bindEvents() {
        document.querySelectorAll('.filter-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
                e.target.classList.add('active');
                this.currentSource = e.target.dataset.source;
                this.loadItems();
            });
        });

        // 问答事件委托
        document.getElementById('items-list').addEventListener('click', (e) => {
            if (e.target.classList.contains('ask-btn')) {
                const itemId = e.target.dataset.itemId;
                const input = document.querySelector(`.ask-input[data-item-id="${itemId}"]`);
                if (input && input.value.trim()) {
                    this.askQuestion(itemId, input.value.trim());
                }
            }
        });

        document.getElementById('items-list').addEventListener('keypress', (e) => {
            if (e.target.classList.contains('ask-input') && e.key === 'Enter') {
                const itemId = e.target.dataset.itemId;
                if (e.target.value.trim()) {
                    this.askQuestion(itemId, e.target.value.trim());
                }
            }
        });
    }

    async askQuestion(itemId, question) {
        const answerDiv = document.getElementById(`answer-${itemId}`);
        const btn = document.querySelector(`.ask-btn[data-item-id="${itemId}"]`);
        const input = document.querySelector(`.ask-input[data-item-id="${itemId}"]`);

        // 显示加载状态
        btn.disabled = true;
        btn.textContent = '...';
        answerDiv.style.display = 'block';
        answerDiv.innerHTML = '<span class="loading-text">AI THINKING</span><span class="loading-dots">...</span>';

        try {
            const resp = await fetch(`/api/items/${itemId}/ask`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ question })
            });

            const data = await resp.json();
            if (resp.ok) {
                answerDiv.innerHTML = `<div class="answer-header">// AI ANSWER</div><div class="answer-content">${this.escapeHtml(data.answer)}</div>`;
                input.value = '';
            } else {
                answerDiv.innerHTML = `<div class="answer-error">// ERROR: ${this.escapeHtml(data.error)}</div>`;
            }
        } catch (err) {
            answerDiv.innerHTML = `<div class="answer-error">// NETWORK ERROR</div>`;
        } finally {
            btn.disabled = false;
            btn.textContent = 'ASK';
        }
    }

    async loadStats() {
        try {
            const resp = await fetch('/api/stats');
            const data = await resp.json();

            document.getElementById('total-count').textContent = data.total || 0;
            document.getElementById('qualified-count').textContent = data.qualified || 0;
            document.getElementById('pass-rate').textContent = (data.pass_rate || 0).toFixed(1) + '%';
            document.getElementById('avg-score').textContent = (data.avg_score || 0).toFixed(1);
        } catch (err) {
            console.error('Failed to load stats:', err);
        }
    }

    async loadItems() {
        const container = document.getElementById('items-list');
        container.innerHTML = `
            <div class="loading">
                <span class="loading-text">LOADING DATA</span>
                <span class="loading-dots">...</span>
            </div>
        `;

        try {
            // 默认显示今日精选，选择来源时显示该来源历史数据
            let url = this.currentSource
                ? `/api/items?limit=50&min_score=70&source=${encodeURIComponent(this.currentSource)}`
                : '/api/items/today';

            const resp = await fetch(url);
            const data = await resp.json();

            if (!data.items || data.items.length === 0) {
                container.innerHTML = `
                    <div class="empty-state">
                        <div class="icon">[ ]</div>
                        <div>NO DATA YET // 等待数据抓取中...</div>
                    </div>
                `;
                return;
            }

            container.innerHTML = data.items.map(item => this.renderItem(item)).join('');
        } catch (err) {
            console.error('Failed to load items:', err);
            container.innerHTML = `
                <div class="empty-state">
                    <div class="icon">[!]</div>
                    <div>NETWORK ERROR // 请稍后重试</div>
                </div>
            `;
        }
    }

    renderItem(item) {
        // 处理多种日期格式
        let date = '--';
        if (item.created_at) {
            try {
                // 尝试直接解析
                let d = new Date(item.created_at);
                // 如果解析失败，尝试其他格式
                if (isNaN(d.getTime())) {
                    // 可能是 Go 的 time.Time 格式: "2006-01-02T15:04:05Z07:00"
                    d = new Date(item.created_at.replace(' ', 'T'));
                }
                if (!isNaN(d.getTime())) {
                    date = d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
                }
            } catch (e) {
                console.warn('Date parse error:', item.created_at);
            }
        } else if (item.CreatedAt) {
            // GORM 默认字段名
            try {
                const d = new Date(item.CreatedAt);
                if (!isNaN(d.getTime())) {
                    date = d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
                }
            } catch (e) {}
        }

        return `
            <div class="item-card" data-item-id="${item.ID}">
                <div class="item-header">
                    <span class="item-source">${item.source}</span>
                    <span class="item-score">${item.total_score}</span>
                </div>
                <div class="item-title">
                    <a href="${item.url}" target="_blank" rel="noopener">${this.escapeHtml(item.title)}</a>
                </div>
                <div class="item-meta">
                    <span>// ${date}</span>
                    ${item.author ? `<span>// ${this.escapeHtml(item.author)}</span>` : ''}
                </div>
                <div class="score-breakdown">
                    <div class="score-bar">
                        <div class="score-bar-fill scarcity" style="width: ${item.scarcity_score * 4}%"></div>
                    </div>
                    <div class="score-bar">
                        <div class="score-bar-fill actionable" style="width: ${item.actionable_score * 4}%"></div>
                    </div>
                    <div class="score-bar">
                        <div class="score-bar-fill leverage" style="width: ${item.leverage_score * 4}%"></div>
                    </div>
                    <div class="score-bar">
                        <div class="score-bar-fill resonance" style="width: ${item.resonance_score * 4}%"></div>
                    </div>
                </div>
                <div class="score-labels">
                    <span>稀缺 ${item.scarcity_score}</span>
                    <span>可操作 ${item.actionable_score}</span>
                    <span>杠杆 ${item.leverage_score}</span>
                    <span>共鸣 ${item.resonance_score}</span>
                </div>
                ${item.score_reason ? `<div class="item-reason">// ${this.escapeHtml(item.score_reason)}</div>` : ''}
                <div class="ask-section">
                    <div class="ask-input-wrapper">
                        <input type="text" class="ask-input" placeholder="有问题？问 AI..." data-item-id="${item.ID}">
                        <button class="ask-btn" data-item-id="${item.ID}">ASK</button>
                    </div>
                    <div class="ask-answer" id="answer-${item.ID}" style="display: none;"></div>
                </div>
            </div>
        `;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// 启动
document.addEventListener('DOMContentLoaded', () => {
    new InfoFilter();
});
