const API = window.location.origin;
let token = localStorage.getItem('clinic_token');
let currentUser = JSON.parse(localStorage.getItem('clinic_user') || '{}');
let doctorsCache = [];
let pollingInterval = null;

const authSection = document.getElementById('auth-section');
const appSection = document.getElementById('app-section');
const authMsg = document.getElementById('auth-msg');
const aptMsg = document.getElementById('apt-msg');
const aptList = document.getElementById('apt-list');
const doctorSelect = document.getElementById('apt-doctor');
const dateInput = document.getElementById('apt-date');
const timeSelect = document.getElementById('apt-time');

function showApp() {
    authSection.style.display = 'none';
    appSection.style.display = 'block';
    const info = document.getElementById('user-info');
    info.textContent = `Пользователь: ${currentUser.first_name || ''} ${currentUser.last_name || ''} | Телефон: ${currentUser.phone || ''}`;
    loadDoctors();
    loadAppointments();
    startPolling();
    setupDateInput();
}

function showAuth() {
    authSection.style.display = 'flex';
    appSection.style.display = 'none';
    token = null;
    currentUser = {};
    localStorage.removeItem('clinic_token');
    localStorage.removeItem('clinic_user');
    authMsg.textContent = '';
    aptMsg.textContent = '';
    stopPolling();
}

function setupDateInput() {
    const today = new Date();
    const tomorrow = new Date(today);
    tomorrow.setDate(tomorrow.getDate() + 1);
    dateInput.min = tomorrow.toISOString().split('T')[0];
    const max = new Date(today);
    max.setDate(max.getDate() + 60);
    dateInput.max = max.toISOString().split('T')[0];
}

async function apiFetch(url, options = {}) {
    if (token) {
        options.headers = { ...options.headers, 'Authorization': `Bearer ${token}` };
    }
    const res = await fetch(`${API}${url}`, options);
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
    }
    return res;
}

async function loadDoctors() {
    try {
        const res = await apiFetch('/doctors');
        doctorsCache = await res.json();
        doctorSelect.innerHTML = '<option value="">-- Выберите врача --</option>';
        doctorsCache.forEach(doc => {
            const opt = document.createElement('option');
            opt.value = doc.name;
            opt.textContent = `${doc.name} (${doc.speciality}, каб. ${doc.office})`;
            opt.dataset.speciality = doc.speciality;
            opt.dataset.office = doc.office;
            doctorSelect.appendChild(opt);
        });
    } catch (e) {
        console.error('Failed to load doctors:', e);
    }
}

async function loadSlots() {
    const doctor = doctorSelect.value;
    const date = dateInput.value;
    if (!doctor || !date) {
        timeSelect.innerHTML = '<option value="">-- Сначала выберите врача и дату --</option>';
        return;
    }
    try {
        const res = await apiFetch(`/slots?doctor=${encodeURIComponent(doctor)}&date=${date}`);
        const slots = await res.json();
        timeSelect.innerHTML = '<option value="">-- Выберите время --</option>';
        if (!Array.isArray(slots) || slots.length === 0) {
            const opt = document.createElement('option');
            opt.disabled = true;
            opt.textContent = 'Нет свободных слотов';
            timeSelect.appendChild(opt);
            return;
        }
        slots.forEach(s => {
            const opt = document.createElement('option');
            const t = new Date(s.datetime);
            opt.value = s.datetime;
            opt.textContent = t.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
            timeSelect.appendChild(opt);
        });
    } catch (e) {
        timeSelect.innerHTML = '<option disabled>Ошибка загрузки</option>';
        aptMsg.textContent = e.message;
        aptMsg.className = 'message error';
    }
}

doctorSelect.addEventListener('change', loadSlots);
dateInput.addEventListener('change', loadSlots);

document.getElementById('register-form').onsubmit = async (e) => {
    e.preventDefault();
    authMsg.textContent = '';
    try {
        const res = await apiFetch('/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                first_name: document.getElementById('reg-fname').value,
                last_name: document.getElementById('reg-lname').value,
                phone: document.getElementById('reg-phone').value,
                email: document.getElementById('reg-email').value,
                password: document.getElementById('reg-pass').value
            })
        });
        currentUser = await res.json();
        authMsg.textContent = 'Регистрация успешна. Теперь войдите в систему.';
        authMsg.className = 'message success';
    } catch (err) {
        authMsg.textContent = err.message;
        authMsg.className = 'message error';
    }
};

document.getElementById('login-form').onsubmit = async (e) => {
    e.preventDefault();
    authMsg.textContent = '';
    try {
        const res = await apiFetch('/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                email: document.getElementById('login-email').value,
                password: document.getElementById('login-pass').value
            })
        });
        const data = await res.json();
        token = data.token;
        currentUser = { 
            email: document.getElementById('login-email').value,
            first_name: '',
            last_name: '',
            phone: ''
        };
        localStorage.setItem('clinic_token', token);
        localStorage.setItem('clinic_user', JSON.stringify(currentUser));
        showApp();
    } catch (err) {
        authMsg.textContent = err.message;
        authMsg.className = 'message error';
    }
};

document.getElementById('apt-form').onsubmit = async (e) => {
    e.preventDefault();
    aptMsg.textContent = '';
    try {
        const selectedOpt = doctorSelect.options[doctorSelect.selectedIndex];
        await apiFetch('/appointments/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                doctor: doctorSelect.value,
                doctor_speciality: selectedOpt.dataset.speciality || '',
                doctor_office: selectedOpt.dataset.office || '',
                date: timeSelect.value
            })
        });
        aptMsg.textContent = 'Запись успешно создана. Ожидание обработки...';
        aptMsg.className = 'message success';
        setTimeout(() => {
            loadAppointments();
            aptMsg.textContent = '';
        }, 1500);
    } catch (err) {
        aptMsg.textContent = err.message;
        aptMsg.className = 'message error';
    }
};

document.getElementById('refresh-btn').onclick = loadAppointments;
document.getElementById('logout-btn').onclick = showAuth;

async function loadAppointments() {
    try {
        const res = await apiFetch('/appointments/list');
        let apps = await res.json();
        
        // Защита от null/undefined
        if (!Array.isArray(apps)) {
            apps = [];
        }

        // Сортировка по дате и времени (от ближайших к дальним)
        apps.sort((a, b) => new Date(a.date) - new Date(b.date));

        aptList.innerHTML = '';
        if (apps.length === 0) {
            aptList.innerHTML = '<li class="empty">У вас пока нет записей</li>';
            return;
        }

        apps.forEach(a => {
            const li = document.createElement('li');
            li.className = `status-${(a.status || 'pending').toLowerCase()}`;
            const d = new Date(a.date);
            const dateStr = d.toLocaleDateString('ru-RU');
            const timeStr = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
            
            li.innerHTML = `
                <div class="apt-header">
                    <strong>${a.doctor}</strong>
                    <span class="apt-status">${a.status}</span>
                </div>
                <div class="apt-details">
                    ${a.doctor_speciality ? `Специализация: ${a.doctor_speciality}` : ''}
                    ${a.doctor_office ? ` | Кабинет: ${a.doctor_office}` : ''}
                </div>
                <div class="apt-date">Дата: ${dateStr} в ${timeStr}</div>
            `;
            aptList.appendChild(li);
        });
    } catch (e) {
        console.error('Failed to load appointments:', e);
    }
}

function startPolling() {
    if (pollingInterval) clearInterval(pollingInterval);
    pollingInterval = setInterval(loadAppointments, 3000);
}

function stopPolling() {
    if (pollingInterval) {
        clearInterval(pollingInterval);
        pollingInterval = null;
    }
}

window.addEventListener('beforeunload', stopPolling);

if (token) {
    showApp();
}