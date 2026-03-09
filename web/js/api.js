// API 配置
const API_BASE_URL = '/api';
const PROFILE_JUST_COMPLETED_TTL_MS = 5 * 60 * 1000;

// API 请求封装
const api = {
    // 通用请求方法
    async request(endpoint, options = {}) {
        const url = `${API_BASE_URL}${endpoint}`;
        const token = storage.get('token');
        const headers = {
            'Content-Type': 'application/json',
            ...(options.headers || {}),
        };
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const config = {
            ...options,
            headers,
        };

        try {
            const response = await fetch(url, config);
            if (response.status === 401) {
                // token 无效或过期，跳转登录页
                clearAuthSessionState({ includeRecorder: true });
                window.location.href = 'login.html';
                return;
            }

            const data = await response.json().catch(() => ({}));
            if (!response.ok) {
                throw new Error(data.error || data.detail || '请求失败');
            }

            return data;
        } catch (error) {
            console.error('API Error:', error);
            throw error;
        }
    },

    // 认证相关
    auth: {
        async login(phone, password) {
            return api.request('/auth/login', {
                method: 'POST',
                body: JSON.stringify({ phone, password }),
            });
        },
    },

    // 用户相关
    user: {
        async get() {
            return api.request('/user/profile');
        },

        async updateSettings(settings) {
            // 后端当前未提供 settings 接口，暂时映射到 profile 更新
            return api.request('/user/profile', {
                method: 'PUT',
                body: JSON.stringify(settings),
            });
        },

        async getProfile() {
            return api.request('/user/profile');
        },

        async completeProfile() {
            // 后端无独立 complete-profile 接口；保留为兼容方法
            return { message: 'ok' };
        },

        async delete(password) {
            return api.request('/user/account', {
                method: 'DELETE',
                body: JSON.stringify({ password }),
            });
        },

        async getEraMemories() {
            const [profile, era] = await Promise.all([
                api.request('/user/profile'),
                api.request('/user/era-memories'),
            ]);

            return {
                birth_year: profile.birth_year,
                hometown: profile.hometown,
                main_city: profile.main_city,
                era_memories_status: era.status,
                era_memories: era.era_memories,
            };
        },

        async regenerateEraMemories() {
            await api.request('/user/era-memories', {
                method: 'POST',
            });
            return api.user.getEraMemories();
        },

        async changePassword(oldPassword, newPassword) {
            return api.request('/user/password', {
                method: 'PUT',
                body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
            });
        },

        async getWelcomeMessages() {
            return api.request('/user/welcome-messages');
        },

        async exportData() {
            return api.request('/user/export');
        },
    },

    // 对话相关
    conversation: {
        async start(payload = {}) {
            return api.request('/conversations', {
                method: 'POST',
                body: JSON.stringify(payload),
            });
        },

        async end(conversationId) {
            return api.request(`/conversations/${conversationId}/end`, {
                method: 'POST',
            });
        },

        async endQuick(conversationId) {
            return api.request(`/conversations/${conversationId}/end-quick`, {
                method: 'POST',
            });
        },

        async get(conversationId) {
            return api.request(`/conversations/${conversationId}`);
        },

        async list() {
            const data = await api.request('/conversations');
            return data.conversations || [];
        },
    },

    // 话题相关
    topic: {
        async getOptions() {
            const data = await api.request('/topics');
            const topics = data.topics || [];
            return {
                options: topics.map(t => ({
                    id: t.id,
                    topic: t.title,
                    greeting: t.greeting || '',
                    context: t.context || '',
                    age_start: t.age_start ?? null,
                    age_end: t.age_end ?? null,
                })),
            };
        },
    },

    // 回忆录相关
    memoir: {
        async list() {
            const data = await api.request('/memoirs');
            return data.memoirs || [];
        },

        async get(memoirId) {
            return api.request(`/memoirs/${memoirId}`);
        },

        async update(memoirId, data) {
            return api.request(`/memoirs/${memoirId}`, {
                method: 'PUT',
                body: JSON.stringify(data),
            });
        },

        async delete(memoirId) {
            return api.request(`/memoirs/${memoirId}`, {
                method: 'DELETE',
            });
        },

        async regenerate(memoirId, perspective = '第一人称') {
            return api.request(`/memoirs/${memoirId}/regenerate`, {
                method: 'POST',
                body: JSON.stringify({ perspective }),
            });
        },
    },
};

// Toast 提示工具
const toast = {
    _container: null,

    _getContainer() {
        if (!this._container) {
            this._container = document.createElement('div');
            this._container.className = 'toast';
            document.body.appendChild(this._container);
        }
        return this._container;
    },

    show(message, type = 'info', duration = 3000) {
        const container = this._getContainer();
        container.textContent = message;
        container.className = `toast toast-${type}`;

        // 移除之前的动画类
        container.classList.remove('hide');

        // 显示
        requestAnimationFrame(() => {
            container.classList.add('show');
        });

        // 自动隐藏
        if (this._timer) clearTimeout(this._timer);
        this._timer = setTimeout(() => {
            container.classList.add('hide');
            container.classList.remove('show');
        }, duration);
    },

    success(message, duration = 3000) {
        this.show(message, 'success', duration);
    },

    error(message, duration = 4000) {
        this.show(message, 'error', duration);
    },

    info(message, duration = 3000) {
        this.show(message, 'info', duration);
    }
};

// 本地存储工具
const storage = {
    get(key) {
        const value = localStorage.getItem(key);
        try {
            return JSON.parse(value);
        } catch {
            return value;
        }
    },

    set(key, value) {
        localStorage.setItem(key, JSON.stringify(value));
    },

    remove(key) {
        localStorage.removeItem(key);
    },
};

function clearTransientChatState(options = {}) {
    const { includeRecorder = false } = options;
    const keys = [
        'currentConversationId',
        'selectedTopic',
        'selectedTopicGreeting',
        'selectedTopicContext',
        'selectedTopicAgeStart',
        'selectedTopicAgeEnd',
        'profileJustCompleted',
    ];

    if (includeRecorder) {
        keys.push('selectedRecorder');
    }

    keys.forEach(key => storage.remove(key));
}

function clearAuthSessionState(options = {}) {
    clearTransientChatState(options);
    storage.remove('token');
    storage.remove('userId');
}

function getProfileJustCompletedState() {
    const state = storage.get('profileJustCompleted');
    if (!state || typeof state !== 'object') {
        if (state !== null) {
            storage.remove('profileJustCompleted');
        }
        return null;
    }

    const userId = typeof state.userId === 'string' ? state.userId : '';
    const completedAt = typeof state.completedAt === 'number' ? state.completedAt : 0;
    if (!userId || completedAt <= 0) {
        storage.remove('profileJustCompleted');
        return null;
    }

    if (Date.now() - completedAt > PROFILE_JUST_COMPLETED_TTL_MS) {
        storage.remove('profileJustCompleted');
        return null;
    }

    return state;
}

function shouldSkipOnboardingForRecentlyCompleted(userId) {
    const state = getProfileJustCompletedState();
    if (!state) {
        return false;
    }

    if (!userId || state.userId !== userId) {
        storage.remove('profileJustCompleted');
        return false;
    }

    return true;
}

function markProfileJustCompleted(userId) {
    if (!userId) {
        storage.remove('profileJustCompleted');
        return;
    }

    storage.set('profileJustCompleted', {
        userId,
        completedAt: Date.now(),
    });
}

window.clearTransientChatState = clearTransientChatState;
window.clearAuthSessionState = clearAuthSessionState;
window.getProfileJustCompletedState = getProfileJustCompletedState;
window.shouldSkipOnboardingForRecentlyCompleted = shouldSkipOnboardingForRecentlyCompleted;
window.markProfileJustCompleted = markProfileJustCompleted;
