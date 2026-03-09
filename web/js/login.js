// 登录页逻辑

// 如果已有 token，直接跳转首页
if (storage.get('token')) {
    window.location.href = 'index.html';
}

async function handleLogin(event) {
    event.preventDefault();

    const phone = document.getElementById('phone').value.trim();
    const password = document.getElementById('password').value;
    const errorEl = document.getElementById('loginError');
    const btn = document.getElementById('loginBtn');

    errorEl.textContent = '';

    if (!phone || !password) {
        toast.error('请输入手机号和密码');
        return;
    }

    btn.disabled = true;
    btn.textContent = '登录中...';

    try {
        const data = await api.auth.login(phone, password);
        const previousUserId = storage.get('userId');

        // 存储 token 和用户信息
        storage.set('token', data.token);
        storage.set('userId', data.user.id);
        if (previousUserId && previousUserId !== data.user.id &&
            typeof window.clearTransientChatState === 'function') {
            window.clearTransientChatState({ includeRecorder: true });
        }

        // 显示成功提示
        toast.success('登录成功，正在跳转...');

        // 延迟跳转，让用户看到提示
        setTimeout(() => {
            window.location.href = 'index.html';
        }, 800);
    } catch (error) {
        // 根据错误类型显示友好提示
        let errorMsg = '登录失败，请稍后重试';
        if (error.message) {
            const msg = error.message.toLowerCase();
            if (msg.includes('invalid phone or password') || msg.includes('密码') || msg.includes('credentials')) {
                errorMsg = '手机号或密码不正确，请检查后重试';
            } else if (msg.includes('not found') || msg.includes('不存在')) {
                errorMsg = '该手机号尚未注册';
            } else if (msg.includes('network') || msg.includes('fetch')) {
                errorMsg = '网络连接失败，请检查网络后重试';
            } else {
                errorMsg = error.message;
            }
        }
        toast.error(errorMsg);
        btn.disabled = false;
        btn.textContent = '登录';
    }
}
