// 实时对话页面逻辑 - 基于豆包实时对话API

// Debug 模式检测 - localhost 自动开启，线上可通过 ?debug=1 手动开启
const urlParams = new URLSearchParams(window.location.search);
const DEBUG_MODE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1' || urlParams.get('debug') === '1';


let conversationId = null;
let isProfileCollectionMode = false;  // 是否是信息收集模式
let autoEndTriggered = false;  // 防止自动结束重复触发

// WebSocket 相关
let ws = null;
let isConnected = false;

// 音频相关
let audioContext = null;
let mediaStream = null;
let audioWorklet = null;
let scriptProcessor = null;
let inputSource = null;
let inputSinkGain = null;
let audioWorkletModuleURL = null;

// 音频播放相关
let playbackContext = null;
let audioQueue = [];
let isPlaying = false;
let nextPlayTime = 0;  // 下一个音频块的播放时间
let gainNode = null;   // 音量控制节点

// 状态
let isRecording = false;
let currentAIResponse = '';  // 累积AI回复文本
let isAISpeaking = false;    // AI是否正在说话
let isFirstTTS = true;       // 是否是第一次 TTS（开场白）
let pendingGreeting = null;  // 后端发来的开场白文本，等 TTS 开始时显示
let shouldResumeRecording = false; // TTS/播放彻底结束后再恢复录音
let lastVoiceDetectedAt = 0; // 最近一次检测到人声时间戳
let sentVoiceSinceLastStop = false;
let vadTimer = null;
let awaitingResponse = false; // 已发送 stop，等待 AI 回复
let userSpeechActive = false; // 当前是否检测到用户正在说话
let voiceBars = [];
let pendingPcmBytes = new Uint8Array(0);
let preSpeechPcmBytes = new Uint8Array(0);

// 配置
const SAMPLE_RATE_INPUT = 16000;   // 输入采样率
const SAMPLE_RATE_OUTPUT = 16000; // 输出采样率（实时 TTS 默认值）
const CHUNK_SIZE = 3200;          // 每次发送的音频块大小
const VAD_SILENCE_MS = 1200;      // 语音结束静音阈值（适当放大，减少截断）
const AUDIO_WORKLET_PROCESSOR_NAME = 'pcm-capture-processor';
const SPEECH_START_RMS = 0.006;   // 超过该阈值判定为开始说话
const SPEECH_END_RMS = 0.0035;    // 低于该阈值并持续静音判定为结束
const VISUAL_NOISE_FLOOR = 0.0035;
const VISUAL_BASE_HEIGHTS = [6, 8, 10, 8, 6];
const VISUAL_GAIN = 30;
const PRE_SPEECH_BUFFER_BYTES = 16000 * 2 * 0.35; // 约 350ms 预录缓冲

// 页面加载
window.onload = async function() {
    // 检查是否已登录
    const token = storage.get('token');
    if (!token) {
        window.location.href = 'login.html';
        return;
    }

    conversationId = storage.get('currentConversationId');

    if (!conversationId) {
        alert('未找到对话');
        goHome();
        return;
    }

    // 检查是否是信息收集模式
    // 注意：如果有 profileJustCompleted 标记，说明信息收集刚完成，不要再进入收集模式
    if (!storage.get('profileJustCompleted')) {
        try {
            const profile = await api.user.getProfile();
            isProfileCollectionMode = !profile.onboarding_completed;
            if (isProfileCollectionMode && DEBUG_MODE) {
                console.log('进入信息收集模式');
            }
        } catch (error) {
            console.error('获取用户信息失败:', error);
        }
    }

    showLoading('正在连接');

    // 初始化音频播放
    initPlayback();

    // 连接 WebSocket
    await connectWebSocket();
};

// 提前请求麦克风权限，触发用户交互以解锁 AudioContext
async function requestMicrophoneEarly() {
    try {
        DEBUG_MODE && console.log('提前请求麦克风权限...');
        mediaStream = await navigator.mediaDevices.getUserMedia({
            audio: {
                sampleRate: SAMPLE_RATE_INPUT,
                channelCount: 1,
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true
            }
        });
        DEBUG_MODE && console.log('麦克风权限已获取');

        // 用户点击了"允许"，这是用户交互，现在可以 resume AudioContext
        if (playbackContext && playbackContext.state === 'suspended') {
            await playbackContext.resume();
            DEBUG_MODE && console.log('AudioContext 已恢复，状态:', playbackContext.state);
        }
    } catch (e) {
        console.error('麦克风权限请求失败:', e);
        updateAIText('无法访问麦克风，请检查权限');
        updateVoiceStatus('麦克风错误');
    }
}

// ========== WebSocket 连接 ==========

// 记录师信息 (使用豆包播客 TTS 支持的音色)
const RECORDER_INFO = {
    female: { name: '小安', speaker: 'zh_female_kefunvsheng_uranus_bigtts' },
    male: { name: '小川', speaker: 'zh_male_shaonianzixin_uranus_bigtts' }
};

async function connectWebSocket() {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';

    // 获取选中的记录师
    const selectedRecorder = storage.get('selectedRecorder') || 'female';
    const recorderInfo = RECORDER_INFO[selectedRecorder] || RECORDER_INFO.female;

    // 获取 token
    const token = storage.get('token');

    // 获取选择的话题信息
    const selectedTopic = storage.get('selectedTopic');
    const selectedGreeting = storage.get('selectedTopicGreeting');
    const selectedContext = storage.get('selectedTopicContext');
    // 使用后清除，避免下次对话重复使用
    storage.remove('selectedTopic');
    storage.remove('selectedTopicGreeting');
    storage.remove('selectedTopicContext');

    // 构建 WebSocket URL，带上音色、记录师名字、对话ID和 token 参数
    const params = new URLSearchParams({
        speaker: recorderInfo.speaker,
        recorder_name: recorderInfo.name,
        conversation_id: conversationId,
        token: token,
    });
    if (isProfileCollectionMode) {
        params.set('mode', 'profile_collection');
    }

    // 如果有选择的话题，添加到参数中
    if (selectedTopic) {
        params.set('topic', selectedTopic);
    }
    if (selectedGreeting) {
        params.set('greeting', selectedGreeting);
    }
    if (selectedContext) {
        params.set('context', selectedContext);
    }

    const wsUrl = `${wsProtocol}://${window.location.host}/api/realtime/dialog?${params.toString()}`;

    if (DEBUG_MODE) {
        console.log('连接 WebSocket:', wsUrl);
        console.log('  - 记录师:', recorderInfo.name);
        console.log('  - 开场白:', selectedGreeting ? '自定义' : '默认');
    }

    try {
        ws = new WebSocket(wsUrl);

        ws.onopen = () => {
            DEBUG_MODE && console.log('WebSocket 已连接');
            isConnected = true;
            updateAIText('连接成功，请先听记录师开场');
            updateVoiceStatus('请稍候');
            requestMicrophoneEarly();
        };

        ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            handleServerMessage(message);
        };

        ws.onerror = (error) => {
            console.error('WebSocket 错误:', error);
            showError('连接失败，请刷新重试');
        };

        ws.onclose = () => {
            DEBUG_MODE && console.log('WebSocket 已关闭');
            isConnected = false;
            stopRecording();
        };

    } catch (error) {
        console.error('WebSocket 连接失败:', error);
        showError('连接失败，请刷新重试');
    }
}

function handleServerMessage(message) {
    // 音频消息量太大，debug 模式也不打印
    if (DEBUG_MODE && message.type !== 'audio' && message.type !== 'tts') {
        console.log('收到消息:', message.type, message);
    }

    switch (message.type) {
        // 兼容旧协议：audio/text/event/status
        case 'audio':
            // 收到音频数据，加入播放队列
            const audioData = base64ToArrayBuffer(message.data);
            queueAudio(audioData, message.sample_rate || SAMPLE_RATE_OUTPUT);
            break;

        // 新协议：tts
        case 'tts':
            isAISpeaking = true;
            awaitingResponse = false;
            sentVoiceSinceLastStop = false;
            userSpeechActive = false;
            setVoiceActive(false);
            updateVoiceStatus('记录师正在说话');
            queueAudio(base64ToArrayBuffer(message.data), message.sample_rate || SAMPLE_RATE_OUTPUT);
            break;

        case 'text':
            // 收到文本
            if (message.text_type === 'asr') {
                // 用户说的话 - 不显示
                DEBUG_MODE && console.log('用户说:', message.content);
            } else if (message.text_type === 'response') {
                // AI 回复文字 - 累积并显示
                if (isAISpeaking && message.content) {
                    currentAIResponse += message.content;  // 累积文本
                    // 过滤掉结束标记
                    const displayText = currentAIResponse.replace('【信息收集完成】', '').replace('###END###', '').trim();
                    updateAIText(displayText);
                    // 检测结束标记，触发自动结束
                    checkEndMarker(currentAIResponse);
                }
            }
            break;

        // 新协议：asr/response/done/error
        case 'asr':
            DEBUG_MODE && console.log('用户说:', message.text);
            break;

        case 'response':
            currentAIResponse = message.text || '';
            // 过滤掉结束标记
            const responseDisplayText = currentAIResponse.replace('###END###', '').trim();
            updateAIText(responseDisplayText);
            // 检测结束标记，触发自动结束
            checkEndMarker(currentAIResponse);
            break;

        case 'done':
            isAISpeaking = false;
            awaitingResponse = false;
            sentVoiceSinceLastStop = false;
            userSpeechActive = false;
            lastVoiceDetectedAt = Date.now();
            setVoiceActive(false);
            updateVoiceStatus('请开始说话');
            shouldResumeRecording = true;
            maybeResumeRecordingAfterPlayback();
            break;

        case 'error':
            if (typeof message.error === 'string' && message.error.includes('ASR provider not available')) {
                showError('ASR 未配置，请在服务端设置 ALIYUN_ACCESS_KEY_ID / ALIYUN_ACCESS_KEY_SECRET / ALIYUN_ASR_APP_KEY');
                break;
            }
            showError(message.error || '对话异常');
            break;

        case 'event':
            handleEvent(message.event, message.payload);
            break;

        case 'status':
            if (message.status === 'error') {
                showError(message.message);
            }
            break;

        case 'debug':
            // Debug 模式下显示调试信息
            if (DEBUG_MODE && message.message) {
                showToast(message.message);
            }
            break;

        case 'greeting_text':
            // 后端发来的开场白文本，等 TTS 开始时直接显示
            pendingGreeting = message.content;
            break;

        case 'profile_collection_complete':
            // 后端 Qwen 验证信息收集完成，自动结束对话
            if (isProfileCollectionMode && !autoEndTriggered) {
                autoEndTriggered = true;
                DEBUG_MODE && console.log('收到后端信息收集完成确认，自动结束对话');
                autoEndProfileCollection();
            }
            break;

        case 'intervention':
            break;
    }
}

function handleEvent(event, payload) {
    DEBUG_MODE && console.log('事件:', event, payload);

    switch (event) {
        case 350:
            // TTS 开始 - AI 开始说话
            isAISpeaking = true;
            awaitingResponse = false;
            sentVoiceSinceLastStop = false;
            userSpeechActive = false;
            currentAIResponse = '';  // 清空，准备接收新回复
            // 第一次 TTS：直接显示后端发来的开场白（不依赖豆包回显）
            if (isFirstTTS && pendingGreeting) {
                currentAIResponse = pendingGreeting;
                updateAIText(pendingGreeting);
                pendingGreeting = null;
            }
            updateVoiceStatus('记录师正在说话');
            setVoiceActive(false);
            break;

        case 359:
            // TTS 结束 - AI 说完了，可以开始录音
            isAISpeaking = false;
            awaitingResponse = false;
            sentVoiceSinceLastStop = false;
            userSpeechActive = false;
            lastVoiceDetectedAt = Date.now();
            // 第一次 TTS 结束后，标记已完成开场白
            if (isFirstTTS) {
                isFirstTTS = false;
            }
            updateVoiceStatus('请开始说话');
            shouldResumeRecording = true;
            maybeResumeRecordingAfterPlayback();
            break;

        case 450:
            // 用户开始说话 - 清空音频队列，音波动起来
            // 不更新上方文字，保持显示AI之前的问题
            clearAudioQueue();
            updateVoiceStatus('正在聆听...');
            // 音波由本地实时音量驱动，不在这里强制激活
            break;

        case 459:
            // 用户说完 - AI 开始处理
            updateVoiceStatus('正在思考...');
            setVoiceActive(false);
            break;

        case 152:
        case 153:
            // 会话结束
            isAISpeaking = false;
            updateAIText('对话已结束');
            updateVoiceStatus('已结束');
            setVoiceActive(false);
            stopRecording();
            break;
    }
}

// ========== 音频录制 ==========

async function startRecording() {
    if (isRecording || !isConnected) return;

    try {
        // 如果还没有 mediaStream（提前请求失败或未执行），现在请求
        if (!mediaStream) {
            mediaStream = await navigator.mediaDevices.getUserMedia({
                audio: {
                    sampleRate: SAMPLE_RATE_INPUT,
                    channelCount: 1,
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                }
            });
        }

        audioContext = new AudioContext({ sampleRate: SAMPLE_RATE_INPUT });
        if (audioContext.state === 'suspended') {
            await audioContext.resume();
        }
        DEBUG_MODE && console.log('录音 AudioContext sampleRate:', audioContext.sampleRate);

        inputSource = audioContext.createMediaStreamSource(mediaStream);
        inputSinkGain = audioContext.createGain();
        inputSinkGain.gain.value = 0;

        // 优先使用 AudioWorklet，避免 ScriptProcessorNode 弃用警告
        if (audioContext.audioWorklet) {
            await audioContext.audioWorklet.addModule(getAudioWorkletModuleURL());
            audioWorklet = new AudioWorkletNode(audioContext, AUDIO_WORKLET_PROCESSOR_NAME, {
                numberOfInputs: 1,
                numberOfOutputs: 1,
                channelCount: 1
            });
            audioWorklet.port.onmessage = (event) => {
                if (!event.data) return;
                handleInputChunk(event.data);
            };

            inputSource.connect(audioWorklet);
            audioWorklet.connect(inputSinkGain);
            inputSinkGain.connect(audioContext.destination);
        } else {
            DEBUG_MODE && console.warn('AudioWorklet 不可用，回退 ScriptProcessor');
            scriptProcessor = audioContext.createScriptProcessor(4096, 1, 1);
            scriptProcessor.onaudioprocess = (e) => {
                handleInputChunk(e.inputBuffer.getChannelData(0));
            };

            inputSource.connect(scriptProcessor);
            scriptProcessor.connect(inputSinkGain);
            inputSinkGain.connect(audioContext.destination);
        }

        isRecording = true;
        updateVoiceStatus('请开始说话');
        setVoiceActive(false);
        lastVoiceDetectedAt = Date.now();
        sentVoiceSinceLastStop = false;
        userSpeechActive = false;

        if (!vadTimer) {
            vadTimer = setInterval(() => {
                if (!isRecording || !isConnected || isAISpeaking) return;
                if (awaitingResponse) return;
                if (!sentVoiceSinceLastStop) return;
                if (Date.now() - lastVoiceDetectedAt < VAD_SILENCE_MS) return;

                // 一次发言结束，通知服务端开始生成回复
                if (ws && ws.readyState === WebSocket.OPEN) {
                    flushPendingPCM(true);
                    awaitingResponse = true;
                    ws.send(JSON.stringify({ type: 'stop' }));
                    updateVoiceStatus('正在思考...');
                }
                sentVoiceSinceLastStop = false;
                userSpeechActive = false;
                setVoiceActive(false);
            }, 300);
        }

    } catch (error) {
        console.error('无法访问麦克风:', error);
        updateAIText('无法访问麦克风，请检查权限');
        updateVoiceStatus('麦克风错误');
    }
}

function stopRecording() {
    isRecording = false;
    awaitingResponse = false;
    userSpeechActive = false;
    sentVoiceSinceLastStop = false;
    pendingPcmBytes = new Uint8Array(0);
    preSpeechPcmBytes = new Uint8Array(0);
    setVoiceActive(false);

    if (audioWorklet) {
        audioWorklet.port.onmessage = null;
        audioWorklet.disconnect();
        audioWorklet = null;
    }

    if (scriptProcessor) {
        scriptProcessor.disconnect();
        scriptProcessor = null;
    }

    if (inputSource) {
        inputSource.disconnect();
        inputSource = null;
    }

    if (inputSinkGain) {
        inputSinkGain.disconnect();
        inputSinkGain = null;
    }

    if (audioContext) {
        audioContext.close();
        audioContext = null;
    }

    if (mediaStream) {
        mediaStream.getTracks().forEach(track => track.stop());
        mediaStream = null;
    }

    if (vadTimer) {
        clearInterval(vadTimer);
        vadTimer = null;
    }
}

function sendAudio(pcmData) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;

    const base64Data = arrayBufferToBase64(pcmData);
    ws.send(JSON.stringify({
        type: 'audio',
        data: base64Data
    }));
}

function queuePCMForSend(pcmInt16) {
    const newBytes = new Uint8Array(pcmInt16.buffer);
    const merged = new Uint8Array(pendingPcmBytes.length + newBytes.length);
    merged.set(pendingPcmBytes, 0);
    merged.set(newBytes, pendingPcmBytes.length);
    pendingPcmBytes = merged;

    while (pendingPcmBytes.length >= CHUNK_SIZE) {
        const chunk = pendingPcmBytes.slice(0, CHUNK_SIZE);
        sendAudio(chunk.buffer);
        pendingPcmBytes = pendingPcmBytes.slice(CHUNK_SIZE);
    }
}

function flushPendingPCM(forceAll = false) {
    if (pendingPcmBytes.length === 0) return;

    while (pendingPcmBytes.length >= CHUNK_SIZE) {
        const chunk = pendingPcmBytes.slice(0, CHUNK_SIZE);
        sendAudio(chunk.buffer);
        pendingPcmBytes = pendingPcmBytes.slice(CHUNK_SIZE);
    }

    if (forceAll && pendingPcmBytes.length > 0) {
        sendAudio(pendingPcmBytes.buffer);
        pendingPcmBytes = new Uint8Array(0);
    }
}

function handleInputChunk(inputData) {
    if (!isRecording || !isConnected || !inputData) return;
    // AI 说话或等待回复期间不送音频，避免误触发和回声干扰
    if (isAISpeaking || awaitingResponse) {
        userSpeechActive = false;
        pendingPcmBytes = new Uint8Array(0);
        preSpeechPcmBytes = new Uint8Array(0);
        setVoiceLevel(0);
        return;
    }

    let energy = 0;
    for (let i = 0; i < inputData.length; i++) {
        energy += inputData[i] * inputData[i];
    }
    const rms = Math.sqrt(energy / inputData.length);
    setVoiceLevel(rms);
    const pcmData = float32ToPCM16(inputData);

    if (!userSpeechActive) {
        preSpeechPcmBytes = appendLimitedBytes(preSpeechPcmBytes, new Uint8Array(pcmData.buffer), PRE_SPEECH_BUFFER_BYTES);
    }

    if (rms >= SPEECH_START_RMS) {
        if (!userSpeechActive) {
            userSpeechActive = true;
            updateVoiceStatus('正在聆听...');
            setVoiceActive(true);
            // 首次触发说话时，把阈值之前的短暂语音一并补发，减少起句丢字
            if (preSpeechPcmBytes.length > 0) {
                queuePCMBytesForSend(preSpeechPcmBytes);
                preSpeechPcmBytes = new Uint8Array(0);
            }
        }
        lastVoiceDetectedAt = Date.now();
        sentVoiceSinceLastStop = true;
    } else if (userSpeechActive && rms >= SPEECH_END_RMS) {
        // 已进入说话态后，使用更低阈值持续刷新活跃时间，避免尾音被提前截断。
        lastVoiceDetectedAt = Date.now();
    }

    // 未触发起说阈值前不发送（仅缓存预录）
    if (!userSpeechActive) {
        return;
    }

    // 一旦进入说话态，持续发送直到 VAD 触发 stop，避免尾音/弱音被截断
    queuePCMForSend(pcmData);
}

function appendLimitedBytes(existing, incoming, maxBytes) {
    const merged = new Uint8Array(existing.length + incoming.length);
    merged.set(existing, 0);
    merged.set(incoming, existing.length);
    if (merged.length <= maxBytes) {
        return merged;
    }
    return merged.slice(merged.length - maxBytes);
}

function queuePCMBytesForSend(bytes) {
    const merged = new Uint8Array(pendingPcmBytes.length + bytes.length);
    merged.set(pendingPcmBytes, 0);
    merged.set(bytes, pendingPcmBytes.length);
    pendingPcmBytes = merged;
    flushPendingPCM(false);
}

function getAudioWorkletModuleURL() {
    if (audioWorkletModuleURL) {
        return audioWorkletModuleURL;
    }

    const processorCode = `
class PCMCaptureProcessor extends AudioWorkletProcessor {
    process(inputs) {
        const input = inputs[0];
        if (input && input[0]) {
            this.port.postMessage(input[0]);
        }
        return true;
    }
}
registerProcessor('${AUDIO_WORKLET_PROCESSOR_NAME}', PCMCaptureProcessor);
    `;

    const blob = new Blob([processorCode], { type: 'application/javascript' });
    audioWorkletModuleURL = URL.createObjectURL(blob);
    return audioWorkletModuleURL;
}

// ========== 音频播放 ==========

function initPlayback() {
    // 创建音频上下文，指定采样率
    playbackContext = new AudioContext({ sampleRate: SAMPLE_RATE_OUTPUT });

    // 创建音量控制节点
    gainNode = playbackContext.createGain();
    gainNode.gain.value = 1.0;
    gainNode.connect(playbackContext.destination);

    DEBUG_MODE && console.log('播放上下文采样率:', playbackContext.sampleRate);
    initVoiceBars();
}

async function queueAudio(audioData, sampleRate) {
    // 确保 AudioContext 处于运行状态
    if (playbackContext.state === 'suspended') {
        await playbackContext.resume();
    }

    audioQueue.push({
        data: audioData,
        sampleRate: sampleRate || SAMPLE_RATE_OUTPUT,
    });
    if (!isPlaying) {
        playNextAudio();
    }
}

function clearAudioQueue() {
    audioQueue = [];
    isPlaying = false;
    nextPlayTime = 0;  // 重置播放时间
}

async function playNextAudio() {
    if (audioQueue.length === 0) {
        isPlaying = false;
        maybeResumeRecordingAfterPlayback();
        return;
    }

    isPlaying = true;

    // 批量处理队列中的音频，使用精确的时间调度
    while (audioQueue.length > 0) {
        const item = audioQueue.shift();
        const audioData = item.data;
        const sampleRate = item.sampleRate || SAMPLE_RATE_OUTPUT;

        try {
            const floatData = pcm16LEToFloat32(audioData);

            if (floatData.length === 0) {
                continue;
            }

            // 应用淡入淡出来减少 click 声
            applyFade(floatData);

            const audioBuffer = playbackContext.createBuffer(1, floatData.length, sampleRate);
            audioBuffer.getChannelData(0).set(floatData);

            const source = playbackContext.createBufferSource();
            source.buffer = audioBuffer;
            source.connect(gainNode);

            // 计算播放时间，确保音频连续
            const currentTime = playbackContext.currentTime;
            if (nextPlayTime < currentTime) {
                // 如果落后了，稍微往后推一点避免立即播放产生的 click
                nextPlayTime = currentTime + 0.01;
            }

            source.start(nextPlayTime);
            nextPlayTime += audioBuffer.duration;

        } catch (error) {
            console.error('播放音频失败:', error);
        }
    }

    isPlaying = false;
    maybeResumeRecordingAfterPlayback();
}

function maybeResumeRecordingAfterPlayback() {
    if (!shouldResumeRecording) return;
    if (isRecording || !isConnected || isAISpeaking || awaitingResponse || isPlaying) return;
    if (audioQueue.length > 0) return;

    shouldResumeRecording = false;
    setTimeout(() => {
        if (shouldResumeRecording || isRecording || !isConnected || isAISpeaking || awaitingResponse || isPlaying || audioQueue.length > 0) {
            return;
        }
        startRecording();
    }, 200);
}

// 应用淡入淡出效果减少音频块之间的 click 声
function applyFade(samples) {
    const fadeLength = Math.min(64, Math.floor(samples.length / 10));

    // 淡入
    for (let i = 0; i < fadeLength; i++) {
        samples[i] *= i / fadeLength;
    }

    // 淡出
    for (let i = 0; i < fadeLength; i++) {
        samples[samples.length - 1 - i] *= i / fadeLength;
    }
}

// ========== 界面控制 ==========

// 显示干预调试气泡（仅 debug + 增强模式）
function showInterventionBubble(triggered, guidance, typeLabel, mechanism, timeout, timedOut) {
    const container = document.getElementById('interventionContainer');
    if (!container) return;

    const bubble = document.createElement('div');
    const time = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

    if (triggered && guidance) {
        bubble.className = 'intervention-bubble';
        const mechanismLabel = mechanism === 'knowledge' ? '知识注入' : '指令注入';
        const tag = typeLabel ? `${typeLabel} · ${mechanismLabel}` : '触发干预';
        bubble.innerHTML = `
            <div class="bubble-header">${time} - ${tag}</div>
            <div class="bubble-content">${guidance}</div>
        `;
    } else if (timeout) {
        bubble.className = 'intervention-bubble intervention-timeout';
        const detail = timedOut && timedOut.length ? timedOut.join('、') : '全部';
        bubble.innerHTML = `
            <div class="bubble-header">${time} - 判断超时</div>
            <div class="bubble-content">超时任务: ${detail}</div>
        `;
    } else {
        bubble.className = 'intervention-bubble no-intervention';
        bubble.innerHTML = `
            <div class="bubble-header">${time}</div>
            <div class="bubble-content">无需干预</div>
        `;
    }

    container.appendChild(bubble);

    // 滚动到最新
    container.scrollTop = container.scrollHeight;

    // 限制气泡数量，超过 10 个删除最旧的
    while (container.children.length > 10) {
        container.removeChild(container.firstChild);
    }

    // 5 秒后淡出非触发的气泡
    if (!triggered) {
        setTimeout(() => {
            bubble.style.animation = 'fadeOut 0.3s ease';
            setTimeout(() => bubble.remove(), 300);
        }, 3000);
    }
}

// 显示轻量提示
function showToast(message) {
    // 创建 toast 元素
    const toast = document.createElement('div');
    toast.className = 'toast-message';
    toast.textContent = message;
    toast.style.cssText = `
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        background: rgba(0, 0, 0, 0.8);
        color: white;
        padding: 16px 24px;
        border-radius: 8px;
        font-size: 16px;
        z-index: 9999;
        animation: fadeIn 0.3s ease;
    `;
    document.body.appendChild(toast);

    // 自动移除
    setTimeout(() => {
        toast.style.animation = 'fadeOut 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, 1200);
}

// 更新 AI 文字内容
function updateAIText(text) {
    document.getElementById('aiText').textContent = text;
}

// 更新用户语音状态
function updateVoiceStatus(text) {
    document.getElementById('voiceStatus').textContent = text;
}

// 设置音波动画状态
function setVoiceActive(active) {
    const visualizer = document.getElementById('voiceVisualizer');
    if (!visualizer) return;
    if (active) {
        visualizer.classList.add('active');
    } else {
        visualizer.classList.remove('active');
        setVoiceLevel(0);
    }
}

function initVoiceBars() {
    const visualizer = document.getElementById('voiceVisualizer');
    if (!visualizer) return;
    voiceBars = Array.from(visualizer.querySelectorAll('.bar'));
    setVoiceLevel(0);
}

function setVoiceLevel(rms) {
    if (!voiceBars || voiceBars.length === 0) return;

    const normalized = Math.max(0, Math.min(1, (rms - VISUAL_NOISE_FLOOR) / 0.03));
    const smooth = Math.pow(normalized, 0.7);
    const active = smooth > 0.02;

    for (let i = 0; i < voiceBars.length; i++) {
        const bar = voiceBars[i];
        const base = VISUAL_BASE_HEIGHTS[i] || 6;
        const distanceToCenter = Math.abs(i - 2);
        const weight = 1 - distanceToCenter * 0.2; // 中间条更敏感
        const height = base + VISUAL_GAIN * smooth * Math.max(0.4, weight);
        bar.style.height = `${active ? height.toFixed(1) : base}px`;
        bar.style.opacity = active ? '0.95' : '0.38';
    }
}

// 简化的状态更新函数
function showLoading(text) {
    updateAIText(text + '...');
    updateVoiceStatus('请稍候');
    setVoiceActive(false);
}

function showError(text) {
    updateAIText('错误: ' + text);
    updateVoiceStatus('连接失败');
    setVoiceActive(false);
}

// ========== 工具函数 ==========

function float32ToPCM16(float32Array) {
    const pcm16 = new Int16Array(float32Array.length);
    for (let i = 0; i < float32Array.length; i++) {
        const s = Math.max(-1, Math.min(1, float32Array[i]));
        pcm16[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
    }
    return pcm16;
}

function pcm16LEToFloat32(arrayBuffer) {
    // 显式按小端序读取 16bit PCM
    const dataView = new DataView(arrayBuffer);
    const numSamples = arrayBuffer.byteLength / 2;
    const float32 = new Float32Array(numSamples);

    for (let i = 0; i < numSamples; i++) {
        // true = little-endian
        const int16 = dataView.getInt16(i * 2, true);
        float32[i] = int16 / 32768.0;
    }
    return float32;
}

function arrayBufferToBase64(buffer) {
    let binary = '';
    const bytes = new Uint8Array(buffer);
    for (let i = 0; i < bytes.byteLength; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
}

function base64ToArrayBuffer(base64) {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
    }
    return bytes.buffer;
}

// ========== 页面控制 ==========

// 检测结束标记
let pendingAutoEnd = false;

function checkEndMarker(text) {
    if (!isProfileCollectionMode || autoEndTriggered) return;
    if (text && text.includes('###END###')) {
        autoEndTriggered = true;
        pendingAutoEnd = true;
        DEBUG_MODE && console.log('检测到结束标记 ###END###，等待语音播放完毕...');
        // 开始轮询检查播放是否完成
        checkAndTriggerAutoEnd();
    }
}

// 检查播放是否完成，完成后触发自动结束
function checkAndTriggerAutoEnd() {
    if (!pendingAutoEnd) return;

    // 计算剩余播放时间
    const currentTime = playbackContext ? playbackContext.currentTime : 0;
    const remainingTime = Math.max(0, nextPlayTime - currentTime);

    if (remainingTime > 0.1 || audioQueue.length > 0 || isAISpeaking) {
        // 还有音频在播放，继续等待
        setTimeout(checkAndTriggerAutoEnd, 300);
    } else {
        // 播放完毕，等 1.5 秒后触发结束
        DEBUG_MODE && console.log('语音播放完毕，1.5秒后结束对话');
        setTimeout(() => {
            pendingAutoEnd = false;
            autoEndProfileCollection();
        }, 1500);
    }
}

// 自动结束信息收集（由AI触发）
async function autoEndProfileCollection() {
    DEBUG_MODE && console.log('自动结束信息收集对话');
    conversationEnded = true;

    stopRecording();

    if (ws) {
        ws.send(JSON.stringify({ type: 'stop' }));
        ws.close();
    }

    updateAIText('正在保存...');
    updateVoiceStatus('请稍候');

    try {
        // 结束对话
        await api.conversation.endQuick(conversationId);
        // 显示欢迎弹窗
        await showWelcomeModal();
    } catch (error) {
        console.error('自动结束对话失败:', error);
        // 出错也显示弹窗，让用户能回到主页
        await showWelcomeModal();
    }
}

let conversationEnded = false;

async function endChat() {
    if (conversationEnded) return;
    if (!confirm('确定要结束这次对话吗？')) {
        return;
    }

    conversationEnded = true;
    stopRecording();

    if (ws) {
        ws.send(JSON.stringify({ type: 'stop' }));
        ws.close();
    }

    updateAIText('正在保存...');
    updateVoiceStatus('请稍候');

    try {
        // 结束对话（后台会处理摘要生成和开场白刷新）
        await api.conversation.endQuick(conversationId);

        if (isProfileCollectionMode) {
            // 信息收集模式：显示欢迎弹窗
            await showWelcomeModal();
        } else {
            // 正常对话模式：后台异步处理摘要和回忆录
            showToast('对话已保存，可在「我的回忆」中查看');
            setTimeout(() => {
                navigateHome();
            }, 1500);
        }
    } catch (error) {
        console.error('结束对话失败:', error);
        alert('操作失败: ' + error.message);
        navigateHome();
    }
}

// 显示欢迎弹窗（信息收集完成后）
async function showWelcomeModal() {
    // 标记“刚完成信息收集”，避免后台提取资料有延迟时反复进入引导
    storage.set('profileJustCompleted', true);

    const modal = document.getElementById('welcomeModal');
    if (modal) {
        modal.style.display = 'flex';
    } else {
        // 如果没有弹窗元素，用 alert 兜底
        alert('很高兴认识您！接下来就可以开始记录您的故事了。');
        goHome();
    }
}

function navigateHome() {
    stopRecording();
    if (ws) {
        ws.close();
    }
    storage.remove('currentConversationId');
    window.location.href = 'index.html';
}

// 兼容旧调用（欢迎弹窗等）
function goHome() {
    navigateHome();
}

window.onbeforeunload = function() {
    stopRecording();
    if (ws) {
        ws.close();
    }
    // 用户直接关闭/刷新页面时，兜底结束对话（keepalive 允许页面关闭后请求继续）
    if (conversationId && !conversationEnded) {
        const token = storage.get('token');
        const url = `${API_BASE_URL}/conversations/${conversationId}/end-quick`;
        try {
            fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                keepalive: true,
            });
        } catch (e) {
            // 忽略，页面即将关闭
        }
    }
};
