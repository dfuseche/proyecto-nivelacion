const API_BASE = 'http://localhost:8080/api';

// Elementos del DOM
const authSection = document.getElementById('authSection');
const dashboardSection = document.getElementById('dashboardSection');
const userControls = document.getElementById('userControls');
const userEmailBadge = document.getElementById('userEmailBadge');

const loginTabBtn = document.getElementById('loginTabBtn');
const registerTabBtn = document.getElementById('registerTabBtn');
const authForm = document.getElementById('authForm');
const authSubmitBtn = document.getElementById('authSubmitBtn');
const emailInput = document.getElementById('emailInput');
const passwordInput = document.getElementById('passwordInput');
const authAlert = document.getElementById('authAlert');
const logoutBtn = document.getElementById('logoutBtn');

const dropZone = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');
const selectedFileDetails = document.getElementById('selectedFileDetails');
const fileNameSpan = document.getElementById('fileName');
const fileSizeSpan = document.getElementById('fileSize');
const uploadBtn = document.getElementById('uploadBtn');

const jobsTableBody = document.getElementById('jobsTableBody');
const refreshJobsBtn = document.getElementById('refreshJobsBtn');

const logsModal = document.getElementById('logsModal');
const closeModalBtn = document.getElementById('closeModalBtn');
const modalCloseBtn = document.getElementById('modalCloseBtn');
const modalJobId = document.getElementById('modalJobId');
const modalStatusBadge = document.getElementById('modalStatusBadge');
const modalLogsContent = document.getElementById('modalLogsContent');
const modalDownloadBtn = document.getElementById('modalDownloadBtn');

// Estado Global de la App
let isRegisterMode = false;
let selectedFile = null;
let pollingInterval = null;
let currentModalJobId = null;

// Inicialización
document.addEventListener('DOMContentLoaded', () => {
    checkSession();
    setupEventListeners();
});

function checkSession() {
    const token = localStorage.getItem('okf_token');
    const email = localStorage.getItem('okf_email');

    if (token && email) {
        userEmailBadge.textContent = email;
        authSection.classList.add('hidden');
        dashboardSection.classList.remove('hidden');
        userControls.style.display = 'flex';
        loadJobs();
        startPolling();
    } else {
        authSection.classList.remove('hidden');
        dashboardSection.classList.add('hidden');
        userControls.style.display = 'none';
        stopPolling();
    }
}

function setupEventListeners() {
    // Auth Tabs
    loginTabBtn.addEventListener('click', () => setAuthMode(false));
    registerTabBtn.addEventListener('click', () => setAuthMode(true));

    // Auth Form
    authForm.addEventListener('submit', handleAuthSubmit);
    logoutBtn.addEventListener('click', handleLogout);

    // File Drop Zone
    dropZone.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', (e) => handleFileSelect(e.target.files[0]));

    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.style.borderColor = 'var(--primary)';
    });
    dropZone.addEventListener('dragleave', () => {
        dropZone.style.borderColor = 'rgba(99, 102, 241, 0.4)';
    });
    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.style.borderColor = 'rgba(99, 102, 241, 0.4)';
        if (e.dataTransfer.files.length > 0) {
            handleFileSelect(e.dataTransfer.files[0]);
        }
    });

    // Upload Action
    uploadBtn.addEventListener('click', handleUpload);
    refreshJobsBtn.addEventListener('click', loadJobs);

    // Modal Events
    closeModalBtn.addEventListener('click', closeModal);
    modalCloseBtn.addEventListener('click', closeModal);
    modalDownloadBtn.addEventListener('click', () => {
        if (currentModalJobId) downloadBundle(currentModalJobId);
    });
}

function setAuthMode(register) {
    isRegisterMode = register;
    authAlert.classList.add('hidden');
    if (register) {
        registerTabBtn.classList.add('active');
        loginTabBtn.classList.remove('active');
        authSubmitBtn.textContent = 'Crear Cuenta Nueva';
    } else {
        loginTabBtn.classList.add('active');
        registerTabBtn.classList.remove('active');
        authSubmitBtn.textContent = 'Entrar a la Plataforma';
    }
}

async function handleAuthSubmit(e) {
    e.preventDefault();
    const email = emailInput.value.trim();
    const password = passwordInput.value.trim();

    const endpoint = isRegisterMode ? `${API_BASE}/auth/register` : `${API_BASE}/auth/login`;

    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Error de autenticación');

        localStorage.setItem('okf_token', data.token);
        localStorage.setItem('okf_email', data.user.email);
        checkSession();
    } catch (err) {
        showAuthAlert(err.message, 'danger');
    }
}

function handleLogout() {
    localStorage.removeItem('okf_token');
    localStorage.removeItem('okf_email');
    checkSession();
}

function showAuthAlert(msg, type) {
    authAlert.textContent = msg;
    authAlert.className = `alert alert-${type}`;
    authAlert.classList.remove('hidden');
}

const ALLOWED_EXTENSIONS = ['.md', '.txt', '.html', '.markdown', '.docx', '.pdf'];

function handleFileSelect(file) {
    if (!file) return;

    const ext = '.' + file.name.split('.').pop().toLowerCase();
    if (!ALLOWED_EXTENSIONS.includes(ext)) {
        alert(`Formato de archivo no permitido (${ext}).\nÚnicamente se permiten documentos: ${ALLOWED_EXTENSIONS.join(', ')}`);
        selectedFile = null;
        selectedFileDetails.classList.add('hidden');
        fileInput.value = '';
        uploadBtn.disabled = true;
        return;
    }

    selectedFile = file;
    fileNameSpan.textContent = file.name;
    fileSizeSpan.textContent = formatBytes(file.size);
    selectedFileDetails.classList.remove('hidden');
    uploadBtn.disabled = false;
}

async function handleUpload() {
    if (!selectedFile) return;

    const token = localStorage.getItem('okf_token');
    const formData = new FormData();
    formData.append('file', selectedFile);

    uploadBtn.disabled = true;
    document.getElementById('uploadBtnText').textContent = 'Enviando...';

    try {
        const res = await fetch(`${API_BASE}/jobs/upload`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });

        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Error al subir archivo');

        // Reset Form
        selectedFile = null;
        selectedFileDetails.classList.add('hidden');
        fileInput.value = '';
        uploadBtn.disabled = true;
        document.getElementById('uploadBtnText').textContent = 'Enviar a Procesamiento Asíncrono';

        loadJobs();
    } catch (err) {
        alert('Error en la carga: ' + err.message);
        uploadBtn.disabled = false;
        document.getElementById('uploadBtnText').textContent = 'Enviar a Procesamiento Asíncrono';
    }
}

async function loadJobs() {
    const token = localStorage.getItem('okf_token');
    if (!token) return;

    try {
        const res = await fetch(`${API_BASE}/jobs`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (!res.ok) return;

        const jobs = await res.json();
        renderJobsTable(jobs);
    } catch (err) {
        console.error('Error cargando trabajos:', err);
    }
}

function renderJobsTable(jobs) {
    if (!jobs || jobs.length === 0) {
        jobsTableBody.innerHTML = `<tr><td colspan="6" class="text-center text-muted">No hay trabajos registrados.</td></tr>`;
        return;
    }

    jobsTableBody.innerHTML = jobs.map(j => {
        const statusBadge = getStatusBadge(j.status);
        const dateStr = new Date(j.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        const isDownloadable = j.status === 'COMPLETED';

        return `
            <tr>
                <td><code>${j.id.substring(0, 8)}...</code></td>
                <td><strong>${escapeHtml(j.original_filename)}</strong></td>
                <td>${statusBadge}</td>
                <td>${j.units_count || 0} u.</td>
                <td>${dateStr}</td>
                <td>
                    <button class="btn btn-outline" onclick="openLogsModal('${j.id}')" style="padding: 0.3rem 0.6rem; font-size: 0.8rem;">Logs</button>
                    ${isDownloadable ? `<button class="btn btn-success" onclick="downloadBundle('${j.id}')" style="padding: 0.3rem 0.6rem; font-size: 0.8rem; margin-left: 0.4rem;">Descargar</button>` : ''}
                </td>
            </tr>
        `;
    }).join('');
}

function getStatusBadge(status) {
    switch (status) {
        case 'PENDING': return '<span class="badge badge-pending">PENDING (En Cola)</span>';
        case 'PROCESSING': return '<span class="badge badge-processing">PROCESSING (Convertiendo)</span>';
        case 'COMPLETED': return '<span class="badge badge-completed">COMPLETED (Válido)</span>';
        case 'FAILED': return '<span class="badge badge-failed">FAILED (Error)</span>';
        case 'INVALID': return '<span class="badge badge-invalid">INVALID (Rechazado)</span>';
        default: return `<span class="badge">${status}</span>`;
    }
}

async function openLogsModal(jobId) {
    currentModalJobId = jobId;
    const token = localStorage.getItem('okf_token');

    modalJobId.textContent = jobId;
    modalStatusBadge.textContent = 'CARGANDO';
    modalStatusBadge.className = 'badge';
    modalLogsContent.textContent = 'Consultando logs de trazabilidad desde el servidor Go...';
    modalDownloadBtn.classList.add('hidden');
    logsModal.classList.remove('hidden');

    try {
        const res = await fetch(`${API_BASE}/jobs/${jobId}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await res.json();

        if (!res.ok) throw new Error(data.error || 'Acceso denegado');

        modalStatusBadge.textContent = data.job.status;
        modalStatusBadge.className = `badge badge-${data.job.status.toLowerCase()}`;

        if (data.job.status === 'COMPLETED') {
            modalDownloadBtn.classList.remove('hidden');
        }

        let logsText = `=== REGISTROS DE TRAZABILIDAD (JOB: ${data.job.id}) ===\n`;
        logsText += `Documento Origen: ${data.job.original_filename}\n`;
        logsText += `Estado Actual: ${data.job.status}\n`;
        if (data.job.error_message) {
            logsText += `Mensaje de Error: ${data.job.error_message}\n`;
        }
        logsText += `--------------------------------------------------\n\n`;

        if (data.logs && data.logs.length > 0) {
            data.logs.forEach(l => {
                const time = new Date(l.created_at).toLocaleTimeString();
                logsText += `[${time}] [${l.step}] [${l.status}] ${l.message}\n`;
            });
        } else {
            logsText += `No hay logs detallados en base de datos.`;
        }

        modalLogsContent.textContent = logsText;
    } catch (err) {
        modalLogsContent.textContent = `[ERROR DE AISLAMIENTO]: ${err.message}`;
    }
}

function closeModal() {
    logsModal.classList.add('hidden');
    currentModalJobId = null;
}

async function downloadBundle(jobId) {
    const token = localStorage.getItem('okf_token');

    try {
        const res = await fetch(`${API_BASE}/jobs/${jobId}/download`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });

        if (!res.ok) {
            const errData = await res.json();
            alert('Error en descarga: ' + errData.error);
            return;
        }

        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `bundle_${jobId.substring(0, 8)}.zip`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);
    } catch (err) {
        alert('Fallo en la descarga: ' + err.message);
    }
}

function startPolling() {
    if (pollingInterval) clearInterval(pollingInterval);
    pollingInterval = setInterval(loadJobs, 3000);
}

function stopPolling() {
    if (pollingInterval) clearInterval(pollingInterval);
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function escapeHtml(text) {
    return text.replace(/[&<>"']/g, function(m) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[m];
    });
}
