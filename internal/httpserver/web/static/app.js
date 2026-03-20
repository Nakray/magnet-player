// Глобальный AbortController для отмены поиска
let searchAbortController = null;

// Поиск
async function performSearch() {
    const input = document.getElementById('searchInput');
    const query = input.value.trim();
    
    if (!query) {
        showStatus('Введите запрос для поиска', 'error');
        return;
    }
    
    const searchBtn = document.getElementById('searchBtn');
    const clearBtn = document.getElementById('cancelBtn');
    const statusEl = document.getElementById('searchStatus');
    
    // Отменяем предыдущий поиск если есть
    if (searchAbortController) {
        searchAbortController.abort();
    }
    
    // Создаём новый AbortController
    searchAbortController = new AbortController();
    
    searchBtn.disabled = true;
    clearBtn.style.display = 'block';
    clearBtn.textContent = '⏹ Отмена';
    statusEl.className = 'status';
    statusEl.textContent = 'Поиск...';
    
    try {
        const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`, {
            signal: searchAbortController.signal
        });
        const data = await response.json();
        
        if (response.ok) {
            showStatus(`Найдено результатов: ${data.count}`, 'success');
            displayResults(data.results);
            clearBtn.style.display = 'none';
        } else {
            showStatus(`Ошибка: ${data.error || 'Неизвестная ошибка'}`, 'error');
            clearBtn.style.display = 'none';
        }
    } catch (err) {
        if (err.name === 'AbortError') {
            showStatus('Поиск отменён', 'success');
        } else {
            showStatus(`Ошибка запроса: ${err.message}`, 'error');
        }
        clearBtn.style.display = 'none';
    } finally {
        searchBtn.disabled = false;
        searchAbortController = null;
    }
}

// Отмена поиска
function cancelSearch() {
    if (searchAbortController) {
        searchAbortController.abort();
        searchAbortController = null;
    }
}

// Очистка поиска
function clearSearch() {
    document.getElementById('searchInput').value = '';
    document.getElementById('resultsSection').style.display = 'none';
    document.getElementById('searchStatus').textContent = '';
    document.getElementById('searchStatus').className = 'status';
    document.getElementById('cancelBtn').style.display = 'none';
}

// Отображение результатов
function displayResults(results) {
    const section = document.getElementById('resultsSection');
    const list = document.getElementById('resultsList');
    
    if (!results || results.length === 0) {
        section.style.display = 'none';
        return;
    }
    
    list.innerHTML = results.map((item, index) => `
        <div class="result-item">
            <div class="result-info">
                <div class="result-title">${escapeHtml(item.title)}</div>
                <div class="result-meta">
                    <span>📊 ${formatSize(item.size)}</span>
                    <span>🌱 ${item.seeders} сидов</span>
                    <span>📥 ${item.peers} пиров</span>
                    <span>📡 ${escapeHtml(item.indexer)}</span>
                </div>
            </div>
            <div class="result-actions">
                <button class="btn-play" onclick="playTorrent('${item.magnet_link}')">
                    ▶ Play
                </button>
                <a class="btn-download" href="${item.magnet_link}" title="Открыть в торрент-клиенте">
                    ⬇ Magnet
                </a>
            </div>
        </div>
    `).join('');
    
    section.style.display = 'block';
}

// Воспроизведение торрента
async function playTorrent(magnetLink) {
    console.log('playTorrent: magnetLink =', magnetLink);
    showStatus('Добавление торрента...', 'success');

    try {
        console.log('playTorrent: sending request to /api/add-magnet');
        const response = await fetch('/api/add-magnet', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ magnet: magnetLink })
        });

        console.log('playTorrent: response status =', response.status);
        const data = await response.json();
        console.log('playTorrent: response data =', data);

        if (response.ok) {
            showStatus(`Торрент добавлен: ${data.hash}`, 'success');
            // Показываем прогрессбар загрузки
            showDownloadProgress(data.hash);
        } else {
            showStatus(`Ошибка: ${data.error || 'Не удалось добавить торрент'}`, 'error');
        }
    } catch (err) {
        console.error('playTorrent: error =', err);
        showStatus(`Ошибка: ${err.message}`, 'error');
    }
}

// Отображение прогресса загрузки
let progressInterval = null;
function showDownloadProgress(hash) {
    const statusEl = document.getElementById('searchStatus');
    statusEl.className = 'status success';
    statusEl.textContent = 'Загрузка... 0%';

    // Создаём прогрессбар
    let progressBar = document.getElementById('downloadProgress');
    if (!progressBar) {
        progressBar = document.createElement('div');
        progressBar.id = 'downloadProgress';
        progressBar.style.cssText = 'width: 100%; height: 4px; background: #e0e0e0; margin-top: 8px; border-radius: 2px; overflow: hidden;';
        progressBar.innerHTML = '<div style="width: 0%; height: 100%; background: #4caf50; transition: width 0.3s;"></div>';
        statusEl.parentNode.insertBefore(progressBar, statusEl.nextSibling);
    }

    const progressFill = progressBar.firstChild;

    // Опрос прогресса
    if (progressInterval) {
        clearInterval(progressInterval);
    }

    progressInterval = setInterval(async () => {
        try {
            const response = await fetch(`/api/progress?hash=${encodeURIComponent(hash)}`);
            const data = await response.json();

            if (response.ok && data.progress !== undefined) {
                const progress = Math.min(100, data.progress).toFixed(1);
                statusEl.textContent = `Загрузка... ${progress}%`;
                progressFill.style.width = `${progress}%`;

                if (progress >= 100) {
                    clearInterval(progressInterval);
                    statusEl.textContent = 'Загрузка завершена!';
                    setTimeout(() => {
                        statusEl.textContent = '';
                        statusEl.className = 'status';
                        if (progressBar && progressBar.parentNode) {
                            progressBar.parentNode.removeChild(progressBar);
                        }
                    }, 2000);
                    loadCacheFiles();
                }
            } else if (!response.ok) {
                // Файл ещё не появился в кэше, продолжаем опрос
                console.log('Progress not available yet');
            }
        } catch (err) {
            console.error('Progress polling error:', err);
        }
    }, 1000);
}

// Загрузка файлов кэша
async function loadCacheFiles() {
    const cacheList = document.getElementById('cacheList');
    cacheList.innerHTML = '<div class="loading">Загрузка...</div>';
    
    try {
        const response = await fetch('/api/files');
        const data = await response.json();
        
        if (response.ok && data.files && data.files.length > 0) {
            cacheList.innerHTML = data.files.map(file => `
                <div class="cache-item">
                    <div class="cache-item-title">
                        📁 ${escapeHtml(file.path)}
                        <br>
                        <small>📊 ${formatSize(file.size)} • 🔗 ${file.hash.substring(0, 12)}...</small>
                    </div>
                    <div class="cache-item-actions">
                        <button class="btn-listen" onclick="listenFile('${file.hash}')">
                            ▶ Слушать
                        </button>
                        <button class="btn-delete" onclick="deleteFile('${file.hash}')">
                            🗑 Удалить
                        </button>
                    </div>
                </div>
            `).join('');
        } else {
            cacheList.innerHTML = '<div class="loading">Кэш пуст</div>';
        }
    } catch (err) {
        cacheList.innerHTML = `<div class="loading">Ошибка загрузки: ${err.message}</div>`;
    }
}

// Прослушивание файла из кэша
function listenFile(hash) {
    const playerSection = document.getElementById('playerSection');
    const audioPlayer = document.getElementById('audioPlayer');
    const trackTitle = document.getElementById('trackTitle');
    
    trackTitle.textContent = `Hash: ${hash}`;
    audioPlayer.src = `/api/stream?hash=${hash}`;
    playerSection.style.display = 'block';
    
    // Прокрутка к плееру
    playerSection.scrollIntoView({ behavior: 'smooth' });
}

// Закрыть плеер
function closePlayer() {
    const playerSection = document.getElementById('playerSection');
    const audioPlayer = document.getElementById('audioPlayer');
    
    audioPlayer.pause();
    audioPlayer.src = '';
    playerSection.style.display = 'none';
}

// Удаление файла
async function deleteFile(hash) {
    if (!confirm('Удалить этот файл из кэша?')) return;
    
    try {
        const response = await fetch(`/api/files/${hash}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            showStatus('Файл удалён', 'success');
            loadCacheFiles();
        } else {
            const data = await response.json();
            showStatus(`Ошибка: ${data.error || 'Не удалось удалить'}`, 'error');
        }
    } catch (err) {
        showStatus(`Ошибка: ${err.message}`, 'error');
    }
}

// Показать статус
function showStatus(message, type) {
    const statusEl = document.getElementById('searchStatus');
    statusEl.textContent = message;
    statusEl.className = `status ${type}`;
    
    // Авто-скрытие через 5 секунд
    setTimeout(() => {
        if (statusEl.textContent === message) {
            statusEl.textContent = '';
            statusEl.className = 'status';
        }
    }, 5000);
}

// Форматирование размера
function formatSize(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    
    return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + units[i];
}

// Экранирование HTML
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Enter для поиска
document.getElementById('searchInput').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        performSearch();
    }
});

// Загрузка кэша при старте
document.addEventListener('DOMContentLoaded', loadCacheFiles);
