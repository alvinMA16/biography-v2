let currentMemoir = null;

window.onload = async function() {
    const token = storage.get('token');
    if (!token) {
        window.location.href = 'login.html';
        return;
    }

    const memoirId = new URLSearchParams(window.location.search).get('id');
    if (!memoirId) {
        alert('未找到回忆录');
        goBack();
        return;
    }

    await loadMemoir(memoirId);
};

async function loadMemoir(memoirId) {
    try {
        currentMemoir = await api.memoir.get(memoirId);

        document.getElementById('memoirTitle').textContent = currentMemoir.title;
        document.getElementById('memoirContent').textContent = currentMemoir.content || '（内容为空）';

        const yearText = formatYearRange(currentMemoir.year_start, currentMemoir.year_end, currentMemoir.time_period);
        document.getElementById('memoirYear').textContent = yearText;
    } catch (error) {
        console.error('加载回忆录失败:', error);
        alert('加载失败: ' + error.message);
        goBack();
    }
}

function formatYearRange(yearStart, yearEnd, timePeriod) {
    const parts = [];

    if (yearStart && yearEnd) {
        if (yearStart === yearEnd) {
            parts.push(`${yearStart}年`);
        } else {
            parts.push(`${yearStart}-${yearEnd}年`);
        }
    } else if (yearStart) {
        parts.push(`${yearStart}年`);
    }

    if (timePeriod) {
        parts.push(timePeriod);
    }

    return parts.join(' · ');
}

function goBack() {
    window.location.href = 'memoir.html';
}
