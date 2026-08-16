const API_BASE = 'http://localhost:8080/api';

// Estado Global de la App
let isRegisterMode = false;
let selectedFile = null;
let pollingInterval = null;
let currentModalJobId = null;

// Estado de Paginación
let allJobs = [];
let currentPage = 1;
let pageSize = 5;

// Helper seguro para obtener elementos del DOM
function getElem(id) {
    return document.getElementById(id);
}

// Inicialización Segura
document.addEventListener('DOMContentLoaded', () => {
    try {
        checkSession();
        setupEventListeners();
    } catch (err) {
        console.error("Error durante la inicialización de la interfaz:", err);
    }
});

function checkSession() {
    const token = localStorage.getItem('okf_token');
    const email = localStorage.getItem('okf_email');

    const authSec = getElem('authSection');
    const dashSec = getElem('dashboardSection');
    const uControls = getElem('userControls');
    const uBadge = getElem('userEmailBadge');

    if (token && email) {
        if (uBadge) uBadge.textContent = email;
        if (authSec) authSec.classList.add('hidden');
        if (dashSec) dashSec.classList.remove('hidden');
        if (uControls) uControls.style.display = 'flex';
        loadJobs();
        startPolling();
    } else {
        if (authSec) authSec.classList.remove('hidden');
        if (dashSec) dashSec.classList.add('hidden');
        if (uControls) uControls.style.display = 'none';
        stopPolling();
    }
}

function setupEventListeners() {
    const loginTabBtn = getElem('loginTabBtn');
    const registerTabBtn = getElem('registerTabBtn');
    const authForm = getElem('authForm');
    const logoutBtn = getElem('logoutBtn');
    const dropZone = getElem('dropZone');
    const fileInput = getElem('fileInput');
    const uploadBtn = getElem('uploadBtn');
    const refreshJobsBtn = getElem('refreshJobsBtn');
    const closeModalBtn = getElem('closeModalBtn');
    const modalCloseBtn = getElem('modalCloseBtn');
    const modalDownloadBtn = getElem('modalDownloadBtn');

    // Auth Tabs
    if (loginTabBtn) loginTabBtn.addEventListener('click', () => setAuthMode(false));
    if (registerTabBtn) registerTabBtn.addEventListener('click', () => setAuthMode(true));

    // Auth Form & Logout
    if (authForm) authForm.addEventListener('submit', handleAuthSubmit);
    if (logoutBtn) logoutBtn.addEventListener('click', handleLogout);

    // File Drop Zone
    if (dropZone && fileInput) {
        dropZone.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', (e) => handleFileSelect(e.target.files[0]));

        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.style.borderColor = 'var(--uniandes-yellow)';
        });
        dropZone.addEventListener('dragleave', () => {
            dropZone.style.borderColor = 'rgba(246, 178, 27, 0.4)';
        });
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.style.borderColor = 'rgba(246, 178, 27, 0.4)';
            if (e.dataTransfer.files.length > 0) {
                handleFileSelect(e.dataTransfer.files[0]);
            }
        });
    }

    // Upload & Refresh Actions
    if (uploadBtn) uploadBtn.addEventListener('click', handleUpload);
    if (refreshJobsBtn) refreshJobsBtn.addEventListener('click', loadJobs);

    // Modal Events
    if (closeModalBtn) closeModalBtn.addEventListener('click', closeModal);
    if (modalCloseBtn) modalCloseBtn.addEventListener('click', closeModal);
    if (modalDownloadBtn) {
        modalDownloadBtn.addEventListener('click', () => {
            if (currentModalJobId) downloadBundle(currentModalJobId);
        });
    }
}

function setAuthMode(register) {
    isRegisterMode = register;
    const authAlert = getElem('authAlert');
    const loginTabBtn = getElem('loginTabBtn');
    const registerTabBtn = getElem('registerTabBtn');
    const authSubmitBtn = getElem('authSubmitBtn');

    if (authAlert) authAlert.classList.add('hidden');
    if (register) {
        if (registerTabBtn) registerTabBtn.classList.add('active');
        if (loginTabBtn) loginTabBtn.classList.remove('active');
        if (authSubmitBtn) authSubmitBtn.textContent = 'Crear Cuenta Nueva';
    } else {
        if (loginTabBtn) loginTabBtn.classList.add('active');
        if (registerTabBtn) registerTabBtn.classList.remove('active');
        if (authSubmitBtn) authSubmitBtn.textContent = 'Entrar a la Plataforma';
    }
}

async function handleAuthSubmit(e) {
    e.preventDefault();
    const emailInput = getElem('emailInput');
    const passwordInput = getElem('passwordInput');

    const email = emailInput ? emailInput.value.trim() : '';
    const password = passwordInput ? passwordInput.value.trim() : '';

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
    const authAlert = getElem('authAlert');
    if (authAlert) {
        authAlert.textContent = msg;
        authAlert.className = `alert alert-${type}`;
        authAlert.classList.remove('hidden');
    }
}

const ALLOWED_EXTENSIONS = ['.md', '.txt', '.html', '.markdown', '.docx', '.pdf'];

function handleFileSelect(file) {
    if (!file) return;

    const fileInput = getElem('fileInput');
    const selectedFileDetails = getElem('selectedFileDetails');
    const fileNameSpan = getElem('fileName');
    const fileSizeSpan = getElem('fileSize');
    const uploadBtn = getElem('uploadBtn');

    const ext = '.' + file.name.split('.').pop().toLowerCase();
    if (!ALLOWED_EXTENSIONS.includes(ext)) {
        alert(`Formato de archivo no permitido (${ext}).\nÚnicamente se permiten documentos: ${ALLOWED_EXTENSIONS.join(', ')}`);
        selectedFile = null;
        if (selectedFileDetails) selectedFileDetails.classList.add('hidden');
        if (fileInput) fileInput.value = '';
        if (uploadBtn) uploadBtn.disabled = true;
        return;
    }

    selectedFile = file;
    if (fileNameSpan) fileNameSpan.textContent = file.name;
    if (fileSizeSpan) fileSizeSpan.textContent = formatBytes(file.size);
    if (selectedFileDetails) selectedFileDetails.classList.remove('hidden');
    if (uploadBtn) uploadBtn.disabled = false;
}

async function handleUpload() {
    if (!selectedFile) return;

    const token = localStorage.getItem('okf_token');
    const formData = new FormData();
    formData.append('file', selectedFile);

    const uploadBtn = getElem('uploadBtn');
    const uploadBtnText = getElem('uploadBtnText');
    const fileInput = getElem('fileInput');
    const selectedFileDetails = getElem('selectedFileDetails');

    if (uploadBtn) uploadBtn.disabled = true;
    if (uploadBtnText) uploadBtnText.textContent = 'Enviando...';

    try {
        const res = await fetch(`${API_BASE}/jobs/upload`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });

        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Error al subir archivo');

        selectedFile = null;
        if (selectedFileDetails) selectedFileDetails.classList.add('hidden');
        if (fileInput) fileInput.value = '';
        if (uploadBtn) uploadBtn.disabled = true;
        if (uploadBtnText) uploadBtnText.textContent = 'Enviar a Procesamiento Asíncrono';

        loadJobs();
    } catch (err) {
        alert('Error en la carga: ' + err.message);
        if (uploadBtn) uploadBtn.disabled = false;
        if (uploadBtnText) uploadBtnText.textContent = 'Enviar a Procesamiento Asíncrono';
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
        allJobs = jobs || [];
        renderJobsTable();
    } catch (err) {
        console.error('Error cargando trabajos:', err);
    }
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

function renderJobsTable() {
    const jobsTableBody = getElem('jobsTableBody');
    const paginationInfo = getElem('paginationInfo');
    const pageIndicator = getElem('pageIndicator');
    const prevPageBtn = getElem('prevPageBtn');
    const nextPageBtn = getElem('nextPageBtn');
    const pageSizeSelect = getElem('pageSizeSelect');

    if (!jobsTableBody) return;

    const totalJobs = allJobs.length;

    if (totalJobs === 0) {
        jobsTableBody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 2rem; color: #94a3b8;">No hay trabajos registrados.</td></tr>`;
        if (paginationInfo) paginationInfo.textContent = `Mostrando 0 de 0 trabajos`;
        if (pageIndicator) pageIndicator.textContent = `Página 1 de 1`;
        if (prevPageBtn) prevPageBtn.disabled = true;
        if (nextPageBtn) nextPageBtn.disabled = true;
        return;
    }

    const totalPages = Math.ceil(totalJobs / pageSize) || 1;
    if (currentPage > totalPages) currentPage = totalPages;
    if (currentPage < 1) currentPage = 1;

    const startIdx = (currentPage - 1) * pageSize;
    const endIdx = Math.min(startIdx + pageSize, totalJobs);
    const pageJobs = allJobs.slice(startIdx, endIdx);

    jobsTableBody.innerHTML = pageJobs.map(j => {
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
                    <div class="actions-cell" style="display: flex; gap: 0.4rem; align-items: center;">
                        <button class="btn btn-outline btn-sm" onclick="window.openLogsModal('${j.id}')">📄 Logs</button>
                        ${isDownloadable ? `<button class="btn btn-success btn-sm" onclick="window.downloadBundle('${j.id}')">⬇️ Descargar</button>` : ''}
                        <button class="btn btn-danger-sm" onclick="window.deleteJob('${j.id}')" style="background-color: rgba(239, 68, 68, 0.25) !important; border: 1px solid #ef4444 !important; color: #fca5a5 !important; padding: 0.4rem 0.75rem !important; border-radius: 0.375rem !important; font-size: 0.825rem !important; font-weight: bold !important; cursor: pointer !important;" title="Eliminar trabajo">🗑️ Eliminar</button>
                    </div>
                </td>
            </tr>
        `;
    }).join('');

    if (paginationInfo) paginationInfo.textContent = `Mostrando ${startIdx + 1} - ${endIdx} de ${totalJobs} trabajos`;
    if (pageIndicator) pageIndicator.textContent = `Página ${currentPage} de ${totalPages}`;
    if (prevPageBtn) prevPageBtn.disabled = currentPage === 1;
    if (nextPageBtn) nextPageBtn.disabled = currentPage >= totalPages;

    if (pageSizeSelect && pageSizeSelect.value !== String(pageSize)) {
        pageSizeSelect.value = String(pageSize);
    }
}

// Exposiciones Globales en Window
window.changePage = function(newPage) {
    const totalPages = Math.ceil(allJobs.length / pageSize) || 1;
    if (newPage >= 1 && newPage <= totalPages) {
        currentPage = newPage;
        renderJobsTable();
    }
};

window.changePageSize = function(newSize) {
    pageSize = parseInt(newSize, 10) || 5;
    currentPage = 1;
    renderJobsTable();
};

window.deleteJob = async function(jobId) {
    if (!confirm('¿Estás seguro de que deseas eliminar este trabajo de conversión y sus archivos asociados?')) {
        return;
    }

    const token = localStorage.getItem('okf_token');

    try {
        const res = await fetch(`${API_BASE}/jobs/${jobId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });

        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Error al eliminar el trabajo');

        loadJobs();
    } catch (err) {
        alert('Fallo al eliminar trabajo: ' + err.message);
    }
};

window.openLogsModal = async function(jobId) {
    currentModalJobId = jobId;
    const token = localStorage.getItem('okf_token');

    const modalJobId = getElem('modalJobId');
    const modalStatusBadge = getElem('modalStatusBadge');
    const modalLogsContent = getElem('modalLogsContent');
    const modalDownloadBtn = getElem('modalDownloadBtn');
    const logsModal = getElem('logsModal');

    if (modalJobId) modalJobId.textContent = jobId;
    if (modalStatusBadge) {
        modalStatusBadge.textContent = 'CARGANDO';
        modalStatusBadge.className = 'badge';
    }
    if (modalLogsContent) modalLogsContent.textContent = 'Consultando logs de trazabilidad desde el servidor Go...';
    if (modalDownloadBtn) modalDownloadBtn.classList.add('hidden');
    if (logsModal) logsModal.classList.remove('hidden');

    try {
        const res = await fetch(`${API_BASE}/jobs/${jobId}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const data = await res.json();

        if (!res.ok) throw new Error(data.error || 'Acceso denegado');

        if (modalStatusBadge) {
            modalStatusBadge.textContent = data.job.status;
            modalStatusBadge.className = `badge badge-${data.job.status.toLowerCase()}`;
        }

        if (data.job.status === 'COMPLETED' && modalDownloadBtn) {
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

        if (modalLogsContent) modalLogsContent.textContent = logsText;
    } catch (err) {
        if (modalLogsContent) modalLogsContent.textContent = `[ERROR DE AISLAMIENTO]: ${err.message}`;
    }
};

window.downloadBundle = async function(jobId) {
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
};

function closeModal() {
    const logsModal = getElem('logsModal');
    if (logsModal) logsModal.classList.add('hidden');
    currentModalJobId = null;
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
