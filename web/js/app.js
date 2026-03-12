// 首页逻辑

// Debug 模式检测
const DEBUG_MODE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';

// ========== 动态欢迎语 ==========

const GENERAL_GREETINGS = ['你好呀', '嗨', '您好', '你好'];

function getTimeGreeting() {
    const hour = new Date().getHours();
    if (hour >= 6 && hour < 11) return '早上好';
    if (hour >= 11 && hour < 13) return '中午好';
    if (hour >= 13 && hour < 18) return '下午好';
    if (hour >= 18 && hour < 23) return '晚上好';
    return '夜深了';
}

function getGreeting() {
    // 约 30% 概率用通用问候，70% 用时间问候
    if (Math.random() < 0.3) {
        return GENERAL_GREETINGS[Math.floor(Math.random() * GENERAL_GREETINGS.length)];
    }
    return getTimeGreeting();
}

function getDisplayName(profile) {
    // 优先 preferred_name，其次 nickname，最后用姓氏+先生/女士
    if (profile.preferred_name) return profile.preferred_name;
    if (profile.nickname) {
        // 如果有 gender，用姓氏+先生/女士
        if (profile.gender && profile.nickname.length >= 1) {
            const surname = profile.nickname.charAt(0);
            const suffix = profile.gender === '女' ? '女士' : '先生';
            return surname + suffix;
        }
        return profile.nickname;
    }
    return '';
}

function updateWelcomeText(profile, messages) {
    const container = document.getElementById('welcomeText');
    if (!container) return;

    const name = getDisplayName(profile);
    const greetWord = getGreeting();
    const greeting = name ? `${greetWord}，${name}。` : `${greetWord}。`;
    if (!messages || messages.length === 0) {
        container.innerHTML = `<p>${greeting}</p>`;
        return;
    }

    const picked = messages[Math.floor(Math.random() * messages.length)];
    const content = picked.content;
    const showGreeting = picked.show_greeting !== false;

    if (showGreeting) {
        container.innerHTML = `<p>${greeting}</p><p style="white-space:pre-line">${content}</p>`;
        return;
    }

    container.innerHTML = `<p style="white-space:pre-line">${content}</p>`;
}

// 初始化应用
async function initApp() {
    // 检查是否已登录
    const token = storage.get('token');
    if (!token) {
        window.location.href = 'login.html';
        return;
    }

    // 如果是 debug 模式，显示时代记忆入口
    if (DEBUG_MODE) {
        const eraMemoriesBtn = document.getElementById('eraMemoriesBtn');
        if (eraMemoriesBtn) {
            eraMemoriesBtn.style.display = 'block';
        }
    }
    // 确保弹窗初始状态是关闭的
    closeRecorderModal();

    // 检查用户是否已完成首次对话
    try {
        const profile = await api.user.getProfile();

        // 动态加载激励语；没有就只显示问候语
        let welcomeMessages = null;
        try {
            const msgs = await api.user.getWelcomeMessages();
            if (msgs && msgs.length > 0) {
                welcomeMessages = msgs.map(m => ({ content: m.content, show_greeting: m.show_greeting }));
            }
        } catch (e) {
            console.warn('加载激励语失败，将只显示问候语:', e);
        }

        // 更新欢迎语
        updateWelcomeText(profile, welcomeMessages);

        if (profile.onboarding_completed) {
            // 用户已完成首次对话，清除本地临时状态，正常显示主页
            if (typeof window.clearRecentFirstSessionCompletion === 'function') {
                window.clearRecentFirstSessionCompletion();
            }
            return;
        }

        // onboarding_completed 还是 false
        // 如果当前用户刚完成首次对话，短时间内先跳过引导，等后台状态同步
        if (typeof window.shouldSkipFirstSessionOnboarding === 'function' &&
            window.shouldSkipFirstSessionOnboarding(storage.get('userId'))) {
            // 首次对话刚结束，暂时跳过引导流程
            return;
        }

        // 未完成首次对话，也没有临时标记，启动引导流程
        startOnboarding();

    } catch (error) {
        console.error('获取用户信息失败:', error);
        // 如果是认证错误，api.js 会自动跳转到登录页
    }
}

// 启动新用户引导流程
function startOnboarding() {
    // 显示记录师选择弹窗，确认后进入首次对话
    const modal = document.getElementById('recorderModal');
    modal.style.display = 'flex';

    // 修改确认按钮的行为
    window.onboardingMode = true;

    // 不默认选中，让用户自己选择
    pendingRecorder = null;
    updateRecorderSelection(null);

    // 预加载音频
    preloadRecorderAudio();
}

// 开始新对话 - 先选择话题
async function startNewChat() {
    const token = storage.get('token');
    if (!token) {
        window.location.href = 'login.html';
        return;
    }

    // 显示话题选择弹窗
    openTopicModal();
}

// ========== 话题选择 ==========

// 话题弹窗标题候选
const TOPIC_MODAL_TITLES = [
    "这次想聊聊哪段经历？",
    "您想从哪里开始讲起？",
    "今天想回忆点什么？",
    "咱们聊聊什么好？",
    "您想讲讲哪方面的事？",
];

const TOPIC_BATCH_SIZE = 3;

function openTopicModal() {
    const modal = document.getElementById('topicModal');
    modal.style.display = 'flex';

    window._topicOptions = [];
    window._topicSeenIds = [];
    window._topicHasMore = true;
    window._topicLoading = false;

    // 随机选择标题
    const title = TOPIC_MODAL_TITLES[Math.floor(Math.random() * TOPIC_MODAL_TITLES.length)];
    document.getElementById('topicModalTitle').textContent = title;

    loadTopicOptions();
}

function closeTopicModal() {
    document.getElementById('topicModal').style.display = 'none';
    window._topicOptions = [];
    window._topicSeenIds = [];
    window._topicHasMore = true;
    window._topicLoading = false;
}

async function loadTopicOptions() {
    if (window._topicLoading) return;

    const container = document.getElementById('topicOptions');
    const currentOptions = window._topicOptions || [];
    const loadingExisting = currentOptions.length > 0;

    window._topicLoading = true;
    if (loadingExisting) {
        renderTopicOptions(currentOptions, true, true);
    } else {
        container.innerHTML = `
            <div class="topic-loading">
                <div class="loading-dots"><span></span><span></span><span></span></div>
                <p>正在回顾您的故事，请稍等</p>
            </div>
        `;
    }

    try {
        const data = await api.topic.nextBatch(window._topicSeenIds || [], TOPIC_BATCH_SIZE);
        const options = data.options || [];

        if (options.length === 0) {
            container.innerHTML = `
                <p class="topic-empty">暂时没有准备好话题，您也可以直接开聊</p>
                <div class="topic-option topic-option-free" onclick="selectFreeTopic()">
                    <div class="topic-title">我有其他想说的</div>
                </div>
            `;
            return;
        }

        const seenIds = new Set(window._topicSeenIds || []);
        options.forEach(opt => {
            if (opt.id) {
                seenIds.add(opt.id);
            }
        });

        window._topicSeenIds = Array.from(seenIds);
        window._topicOptions = options;
        window._topicHasMore = data.has_more === true;

        renderTopicOptions(options, window._topicHasMore, false);

    } catch (error) {
        console.error('加载话题失败:', error);
        container.innerHTML = '<p class="topic-empty">加载失败，请稍后重试</p>';
    } finally {
        window._topicLoading = false;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function renderTopicOptions(options, hasMore, isRefreshing) {
    const container = document.getElementById('topicOptions');
    const refreshDisabled = isRefreshing || !hasMore;
    const refreshLabel = isRefreshing ? '正在换一批...' : (hasMore ? '换一批' : '先看看这些');

    container.innerHTML = options.map((opt, index) => `
        <div class="topic-option" onclick="selectTopicByIndex(${index})">
            <div class="topic-title">${escapeHtml(opt.topic)}</div>
        </div>
    `).join('') + `
        <div class="topic-option topic-option-free" onclick="selectFreeTopic()">
            <div class="topic-title">我有其他想说的</div>
        </div>
        <div class="topic-actions">
            <button class="btn btn-secondary topic-refresh-btn" onclick="loadTopicOptions()" ${refreshDisabled ? 'disabled' : ''}>${refreshLabel}</button>
        </div>
    `;
}

// 通过索引选择话题（避免 HTML 转义问题）
async function selectTopicByIndex(index) {
    const options = window._topicOptions || [];
    if (index < 0 || index >= options.length) return;

    const opt = options[index];
    await selectTopic(opt.id, opt.topic, opt.greeting, opt.context, opt.source, opt.age_start, opt.age_end);
}

async function selectFreeTopic() {
    await selectTopic(null, '__free__', '好的呀，今天我听您的。', '', '', null, null);
}

async function selectTopic(topicId, topic, greeting, context, topicSource, ageStart, ageEnd) {
    closeTopicModal();

    try {
        // 创建新对话
        const conversationInput = {};
        if (topic && topic !== '__free__') {
            conversationInput.topic = topic;
            conversationInput.greeting = greeting || '';
            conversationInput.context = context || '';
            if (topicId) {
                conversationInput.topic_id = topicId;
                conversationInput.topic_source = topicSource || '';
            }
        }
        const result = await api.conversation.start(conversationInput);
        storage.set('currentConversationId', result.id);

        // 存储选择的话题信息
        storage.set('selectedTopic', topic);
        storage.set('selectedTopicGreeting', greeting);
        storage.set('selectedTopicContext', context || '');
        if (topicId) {
            storage.set('selectedTopicId', topicId);
        } else {
            storage.remove('selectedTopicId');
        }
        if (topicSource) {
            storage.set('selectedTopicSource', topicSource);
        } else {
            storage.remove('selectedTopicSource');
        }
        // 存储年龄范围（用于截取时代记忆）
        if (ageStart !== null && ageStart !== undefined) {
            storage.set('selectedTopicAgeStart', ageStart);
        } else {
            storage.remove('selectedTopicAgeStart');
        }
        if (ageEnd !== null && ageEnd !== undefined) {
            storage.set('selectedTopicAgeEnd', ageEnd);
        } else {
            storage.remove('selectedTopicAgeEnd');
        }

        // 跳转到对话页面
        window.location.href = 'chat.html';
    } catch (error) {
        console.error('开始对话失败:', error);
        alert('开始对话失败: ' + error.message);
    }
}

// 查看回忆录
function viewMemoirs() {
    window.location.href = 'memoir.html';
}

// 切换下拉菜单
function toggleDropdown() {
    const dropdown = document.getElementById('userDropdown');
    dropdown.classList.toggle('show');
}

// 退出登录
function logout() {
    if (confirm('确定要退出登录吗？')) {
        if (typeof window.clearAuthSessionState === 'function') {
            window.clearAuthSessionState({ includeRecorder: true });
        }
        window.location.href = 'login.html';
    }
}

// 注销账号（删除服务器上的所有数据）
async function deleteAccount() {
    if (!confirm('确定要注销账号吗？\n\n注销后所有数据将被立即删除，且不可恢复。')) {
        return;
    }

    const password = prompt('请输入当前密码以确认注销');
    if (!password) {
        return;
    }

    try {
        await api.user.delete(password);
        if (typeof window.clearAuthSessionState === 'function') {
            window.clearAuthSessionState({ includeRecorder: true });
        }
        alert('账号已注销');
        window.location.href = 'login.html';
    } catch (error) {
        console.error('注销账号失败:', error);
        alert('注销失败: ' + error.message);
    }
}

// ========== 数据导出 ==========

async function exportMyData() {
    const dropdown = document.getElementById('userDropdown');
    dropdown.classList.remove('show');

    try {
        const data = await api.user.exportData();
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = '我的回忆录数据.json';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    } catch (error) {
        console.error('导出数据失败:', error);
        alert('导出失败: ' + error.message);
    }
}

// ========== 修改密码 ==========

function openChangePassword() {
    const dropdown = document.getElementById('userDropdown');
    dropdown.classList.remove('show');

    document.getElementById('oldPassword').value = '';
    document.getElementById('newPassword').value = '';
    document.getElementById('confirmPassword').value = '';
    document.getElementById('changePasswordModal').style.display = 'flex';
}

function closeChangePassword() {
    document.getElementById('changePasswordModal').style.display = 'none';
}

async function submitChangePassword() {
    const oldPwd = document.getElementById('oldPassword').value;
    const newPwd = document.getElementById('newPassword').value;
    const confirmPwd = document.getElementById('confirmPassword').value;

    if (!oldPwd || !newPwd || !confirmPwd) {
        alert('请填写所有密码字段');
        return;
    }
    if (newPwd.length < 6) {
        alert('新密码至少需要6位');
        return;
    }
    if (newPwd !== confirmPwd) {
        alert('两次输入的新密码不一致');
        return;
    }

    try {
        await api.user.changePassword(oldPwd, newPwd);
        alert('密码修改成功');
        closeChangePassword();
    } catch (error) {
        alert(error.message || '密码修改失败');
    }
}

// ========== 记录师选择 ==========

const RECORDERS = {
    female: {
        name: '忆安',
        speaker: 'zh_female_tianmeixiaoyuan_uranus_bigtts',
        // 预览用的简短招呼
        previewGreeting: '您好，我是忆安。很高兴能成为您的人生记录师，期待听您讲述那些珍贵的回忆。'
    },
    male: {
        name: '言川',
        speaker: 'zh_male_shaonianzixin_uranus_bigtts',
        // 预览用的简短招呼
        previewGreeting: '您好，我是言川。能够记录您的人生故事，是我的荣幸。请慢慢讲，我都在听。'
    }
};

// 打开记录师选择弹窗
function openRecorderSelect() {
    const dropdown = document.getElementById('userDropdown');
    dropdown.classList.remove('show');

    const modal = document.getElementById('recorderModal');
    modal.style.display = 'flex';

    // 不默认选中，让用户必须手动点击
    pendingRecorder = null;
    updateRecorderSelection(null);

    // 预加载音频
    preloadRecorderAudio();
}

// 关闭记录师选择弹窗
function closeRecorderModal() {
    document.getElementById('recorderModal').style.display = 'none';
    stopPreviewAudio();
}

// 更新记录师选择的高亮状态
function updateRecorderSelection(gender) {
    document.getElementById('recorderFemale').classList.toggle('selected', gender === 'female');
    document.getElementById('recorderMale').classList.toggle('selected', gender === 'male');
}

// 选择记录师
let previewAudio = null;
let pendingRecorder = null;  // 待确认的选择
let preloadedAudio = {};     // 预加载的音频缓存

async function selectRecorder(gender) {
    pendingRecorder = gender;
    updateRecorderSelection(gender);

    // 播放开场白预览
    playRecorderGreeting(gender);
}

// 确认选择记录师
async function confirmRecorder() {
    // 必须选择一个记录师
    if (!pendingRecorder) {
        alert('请先选择一位记录师');
        return;
    }

    storage.set('selectedRecorder', pendingRecorder);
    stopPreviewAudio();
    closeRecorderModal();

    // 如果是引导模式，进入首次对话
    if (window.onboardingMode) {
        window.onboardingMode = false;
        await startFirstSession();
    }
}

// 进入首次对话
async function startFirstSession() {
    try {
        // 创建新对话
        const result = await api.conversation.start();
        storage.set('currentConversationId', result.id);
        // 跳转到对话页面（会自动检测到未完成首次对话）
        window.location.href = 'chat.html';
    } catch (error) {
        console.error('开始首次对话失败:', error);
        alert('开始对话失败: ' + error.message);
    }
}

// 预加载记录师音频
async function preloadRecorderAudio() {
    // 并行预加载两个记录师的音频
    const genders = ['female', 'male'];
    await Promise.all(genders.map(async (gender) => {
        if (preloadedAudio[gender]) return; // 已加载过

        const recorder = RECORDERS[gender];
        const url = `/api/realtime/preview?speaker=${encodeURIComponent(recorder.speaker)}&text=${encodeURIComponent(recorder.previewGreeting)}`;

        try {
            const resp = await fetch(url);
            if (!resp.ok) {
                console.error('预加载音频失败:', gender, resp.status);
                return;
            }
            const data = await resp.json();
            if (!data.audio) {
                console.error('预加载音频数据为空:', gender);
                return;
            }
            // 缓存 base64 音频数据
            preloadedAudio[gender] = 'data:audio/mp3;base64,' + data.audio;
        } catch (error) {
            console.error('预加载音频失败:', gender, error);
        }
    }));
}

// 播放记录师开场白预览
function playRecorderGreeting(gender) {
    stopPreviewAudio();

    // 优先使用预加载的音频
    if (preloadedAudio[gender]) {
        previewAudio = new Audio(preloadedAudio[gender]);
        previewAudio.play().catch(err => console.error('播放失败:', err));
        return;
    }

    // 兜底：实时加载（预加载失败时）
    const recorder = RECORDERS[gender];
    const url = `/api/realtime/preview?speaker=${encodeURIComponent(recorder.speaker)}&text=${encodeURIComponent(recorder.previewGreeting)}`;

    fetch(url)
        .then(resp => resp.json())
        .then(data => {
            if (!data.audio) return;
            const audioSrc = 'data:audio/mp3;base64,' + data.audio;
            preloadedAudio[gender] = audioSrc; // 缓存起来
            previewAudio = new Audio(audioSrc);
            previewAudio.play().catch(err => console.error('播放失败:', err));
        })
        .catch(err => console.error('播放开场白失败:', err));
}

function stopPreviewAudio() {
    if (previewAudio) {
        previewAudio.pause();
        previewAudio.src = '';
        previewAudio = null;
    }
}

// ========== 时代记忆 ==========

async function viewEraMemories() {
    const dropdown = document.getElementById('userDropdown');
    dropdown.classList.remove('show');

    const modal = document.getElementById('eraMemoriesModal');
    modal.style.display = 'flex';

    // 加载时代记忆
    await loadEraMemories();
}

function closeEraMemoriesModal() {
    stopEraMemoriesPolling();
    document.getElementById('eraMemoriesModal').style.display = 'none';
}

// 轮询检查时代记忆状态
let eraMemoriesPollingTimer = null;

async function loadEraMemories() {
    const contentEl = document.getElementById('eraMemoriesContent');
    const statusEl = document.getElementById('eraMemoriesStatus');
    const birthYearEl = document.getElementById('eraInfoBirthYear');
    const hometownEl = document.getElementById('eraInfoHometown');
    const mainCityEl = document.getElementById('eraInfoMainCity');
    const regenerateBtn = document.getElementById('regenerateBtn');

    contentEl.innerHTML = '<p class="loading-text">加载中...</p>';

    try {
        const data = await api.user.getEraMemories();

        // 更新用户信息
        birthYearEl.textContent = data.birth_year || '-';
        hometownEl.textContent = data.hometown || '-';
        mainCityEl.textContent = data.main_city || '-';

        // 根据状态显示不同内容
        const status = data.era_memories_status || 'none';
        updateEraMemoriesStatus(status);

        if (status === 'completed' && data.era_memories) {
            contentEl.textContent = data.era_memories;
            regenerateBtn.disabled = false;
            regenerateBtn.textContent = '重新生成';
            stopEraMemoriesPolling();
        } else if (status === 'generating') {
            contentEl.innerHTML = '<p class="loading-text">正在生成中，请稍候...</p>';
            regenerateBtn.disabled = true;
            regenerateBtn.textContent = '生成中...';
            startEraMemoriesPolling();
        } else if (status === 'pending') {
            contentEl.innerHTML = '<p class="empty-text">已收集基础信息，等待生成时代记忆</p>';
            regenerateBtn.disabled = false;
            regenerateBtn.textContent = '开始生成';
            stopEraMemoriesPolling();
        } else if (status === 'failed') {
            contentEl.innerHTML = '<p class="empty-text">生成失败，请点击下方按钮重试</p>';
            regenerateBtn.disabled = false;
            regenerateBtn.textContent = '重新生成';
            stopEraMemoriesPolling();
        } else {
            // none - 未收集基础信息
            contentEl.innerHTML = '<p class="empty-text">请先完善基础资料后再查看</p>';
            regenerateBtn.disabled = true;
            regenerateBtn.textContent = '重新生成';
            stopEraMemoriesPolling();
        }
    } catch (error) {
        console.error('加载时代记忆失败:', error);
        contentEl.innerHTML = '<p class="empty-text">加载失败，请稍后重试</p>';
    }
}

function updateEraMemoriesStatus(status) {
    const statusEl = document.getElementById('eraMemoriesStatus');
    const statusMap = {
        'none': { text: '未收集', class: 'status-none' },
        'pending': { text: '待生成', class: 'status-pending' },
        'generating': { text: '生成中', class: 'status-generating' },
        'completed': { text: '已完成', class: 'status-completed' },
        'failed': { text: '生成失败', class: 'status-failed' }
    };
    const info = statusMap[status] || statusMap['none'];
    statusEl.textContent = info.text;
    statusEl.className = 'era-status-badge ' + info.class;
}

function startEraMemoriesPolling() {
    if (eraMemoriesPollingTimer) return;
    eraMemoriesPollingTimer = setInterval(() => {
        loadEraMemories();
    }, 3000);
}

function stopEraMemoriesPolling() {
    if (eraMemoriesPollingTimer) {
        clearInterval(eraMemoriesPollingTimer);
        eraMemoriesPollingTimer = null;
    }
}

async function regenerateEraMemories() {
    const contentEl = document.getElementById('eraMemoriesContent');
    const regenerateBtn = document.getElementById('regenerateBtn');

    contentEl.innerHTML = '<p class="loading-text">正在生成...</p>';
    updateEraMemoriesStatus('generating');
    regenerateBtn.disabled = true;
    regenerateBtn.textContent = '生成中...';

    try {
        const data = await api.user.regenerateEraMemories();

        if (data.era_memories) {
            contentEl.textContent = data.era_memories;
            updateEraMemoriesStatus('completed');
            regenerateBtn.disabled = false;
            regenerateBtn.textContent = '重新生成';
        } else {
            contentEl.innerHTML = '<p class="empty-text">生成失败，请确保已填写出生年份</p>';
            updateEraMemoriesStatus('failed');
            regenerateBtn.disabled = false;
            regenerateBtn.textContent = '重新生成';
        }
    } catch (error) {
        console.error('重新生成时代记忆失败:', error);
        contentEl.innerHTML = '<p class="empty-text">生成失败: ' + error.message + '</p>';
        updateEraMemoriesStatus('failed');
        regenerateBtn.disabled = false;
        regenerateBtn.textContent = '重新生成';
    }
}
