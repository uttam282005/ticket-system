/**
 * Ledger Ticket System Client
 * Pure modern vanilla JS implementing UI-SPEC.md
 */

(() => {
  'use strict';

  // --- State Management ---
  const state = {
    token: localStorage.getItem('token') || '',
    user: JSON.parse(localStorage.getItem('user') || 'null'),
    tickets: [],
    currentTicket: null,
    authMode: 'login', // 'login' | 'register'
    theme: localStorage.getItem('theme') || 'auto',
  };

  // --- DOM Elements ---
  const elements = {
    app: document.getElementById('app'),
    brandLink: document.getElementById('brand-link'),
    userEmail: document.getElementById('user-email'),
    themeToggle: document.getElementById('theme-toggle'),
    topNewTicketBtn: document.getElementById('top-new-ticket-btn'),
    signOutBtn: document.getElementById('sign-out-btn'),
    
    // Views
    authView: document.getElementById('auth-view'),
    ticketsView: document.getElementById('tickets-view'),
    detailView: document.getElementById('detail-view'),

    // Auth
    authTitle: document.getElementById('auth-title'),
    authForm: document.getElementById('auth-form'),
    authEmail: document.getElementById('auth-email'),
    authPassword: document.getElementById('auth-password'),
    authEmailError: document.getElementById('auth-email-error'),
    authPasswordError: document.getElementById('auth-password-error'),
    authGeneralError: document.getElementById('auth-general-error'),
    authSubmitBtn: document.getElementById('auth-submit-btn'),
    authModeToggle: document.getElementById('auth-mode-toggle'),

    // Tickets List
    openNewTicketBtn: document.getElementById('open-new-ticket-btn'),
    newTicketPanel: document.getElementById('new-ticket-panel'),
    cancelNewTicketBtn: document.getElementById('cancel-new-ticket-btn'),
    newTicketForm: document.getElementById('new-ticket-form'),
    ticketTitle: document.getElementById('ticket-title'),
    ticketDescription: document.getElementById('ticket-description'),
    ticketTitleError: document.getElementById('ticket-title-error'),
    newTicketGeneralError: document.getElementById('new-ticket-general-error'),
    createTicketBtn: document.getElementById('create-ticket-btn'),
    ticketList: document.getElementById('ticket-list'),
    emptyState: document.getElementById('empty-state'),
    emptyNewTicketBtn: document.getElementById('empty-new-ticket-btn'),

    // Detail View
    backToTicketsBtn: document.getElementById('back-to-tickets-btn'),
    detailTitle: document.getElementById('detail-title'),
    detailId: document.getElementById('detail-id'),
    detailTimestamp: document.getElementById('detail-timestamp'),
    detailStatusDot: document.getElementById('detail-status-dot'),
    detailStatusText: document.getElementById('detail-status-text'),
    detailDescription: document.getElementById('detail-description'),
    detailActionBtn: document.getElementById('detail-action-btn'),
    detailCloseDirectBtn: document.getElementById('detail-close-direct-btn'),
    detailClosedNotice: document.getElementById('detail-closed-notice'),
  };

  // --- Helper: Relative Time Formatter ---
  function formatRelativeTime(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    const now = new Date();
    const diffSec = Math.round((now - date) / 1000);

    if (diffSec < 60) return 'just now';
    const diffMin = Math.round(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHours = Math.round(diffMin / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    const diffDays = Math.round(diffHours / 24);
    if (diffDays < 30) return `${diffDays}d ago`;
    const diffMonths = Math.round(diffDays / 30);
    return `${diffMonths}mo ago`;
  }

  function getReadableStatus(status) {
    if (status === 'in_progress') return 'in progress';
    return status || 'open';
  }

  // --- Theme Management ---
  function isDarkTheme() {
    const attr = document.documentElement.getAttribute('data-theme');
    if (attr === 'dark') return true;
    if (attr === 'light') return false;
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  }

  function updateThemeUI() {
    const dark = isDarkTheme();
    elements.themeToggle.textContent = dark ? 'Light' : 'Dark';
    elements.themeToggle.setAttribute('aria-label', dark ? 'Switch to light mode' : 'Switch to dark mode');
  }

  function toggleTheme() {
    const currentDark = isDarkTheme();
    const nextTheme = currentDark ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', nextTheme);
    localStorage.setItem('theme', nextTheme);
    state.theme = nextTheme;
    updateThemeUI();
  }

  if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (!localStorage.getItem('theme')) {
        updateThemeUI();
      }
    });
  }

  // --- API Client ---
  async function api(path, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };

    if (state.token) {
      headers['Authorization'] = `Bearer ${state.token}`;
    }

    try {
      const res = await fetch(path, {
        ...options,
        headers,
      });

      const data = await res.json().catch(() => ({}));

      if (!res.ok) {
        if (res.status === 401 && state.token) {
          // Token expired or invalid
          signOut();
          showAuthError('Session expired. Please sign in again.');
          throw new Error('Unauthorized');
        }
        const message = data.error || `Error ${res.status}`;
        const err = new Error(message);
        err.status = res.status;
        err.data = data;
        throw err;
      }

      return data;
    } catch (err) {
      if (err.message === 'Failed to fetch') {
        throw new Error('Network error. Unable to reach server.');
      }
      throw err;
    }
  }

  // --- Screen Navigation ---
  function showView(view) {
    [elements.authView, elements.ticketsView, elements.detailView].forEach((v) => {
      if (v === view) {
        v.classList.remove('hidden');
      } else {
        v.classList.add('hidden');
      }
    });

    // Update top-bar controls
    if (state.token) {
      elements.userEmail.textContent = state.user?.email || '';
      elements.signOutBtn.classList.remove('hidden');
      if (view === elements.ticketsView) {
        elements.topNewTicketBtn.classList.remove('hidden');
      } else {
        elements.topNewTicketBtn.classList.add('hidden');
      }
    } else {
      elements.userEmail.textContent = '';
      elements.signOutBtn.classList.add('hidden');
      elements.topNewTicketBtn.classList.add('hidden');
    }
  }

  // --- Authentication ---
  function setAuthMode(mode) {
    state.authMode = mode;
    clearAuthErrors();
    if (mode === 'login') {
      elements.authTitle.textContent = 'Sign in';
      elements.authSubmitBtn.textContent = 'Sign in';
      elements.authModeToggle.textContent = 'No account? Create one';
    } else {
      elements.authTitle.textContent = 'Create account';
      elements.authSubmitBtn.textContent = 'Create account';
      elements.authModeToggle.textContent = 'Already have an account? Sign in';
    }
  }

  function clearAuthErrors() {
    elements.authEmailError.textContent = '';
    elements.authPasswordError.textContent = '';
    elements.authGeneralError.textContent = '';
  }

  function showAuthError(msg) {
    elements.authGeneralError.textContent = msg;
  }

  async function handleAuthSubmit(e) {
    e.preventDefault();
    clearAuthErrors();

    const email = elements.authEmail.value.trim();
    const password = elements.authPassword.value;

    let hasError = false;
    if (!email) {
      elements.authEmailError.textContent = 'Email is required';
      hasError = true;
    } else if (!email.includes('@') || !email.includes('.')) {
      elements.authEmailError.textContent = 'Please enter a valid email address';
      hasError = true;
    }

    if (!password) {
      elements.authPasswordError.textContent = 'Password is required';
      hasError = true;
    }

    if (hasError) return;

    const originalBtnText = elements.authSubmitBtn.textContent;
    elements.authSubmitBtn.disabled = true;
    elements.authSubmitBtn.textContent = state.authMode === 'login' ? 'Signing in…' : 'Creating account…';

    try {
      if (state.authMode === 'register') {
        const regRes = await api('/auth/register', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
        // Auto-login after registration
        const loginRes = await api('/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
        loginSuccess(loginRes.token, { id: regRes.id, email: regRes.email });
      } else {
        const loginRes = await api('/auth/login', {
          method: 'POST',
          body: JSON.stringify({ email, password }),
        });
        loginSuccess(loginRes.token, { email });
      }
    } catch (err) {
      showAuthError(err.message);
    } finally {
      elements.authSubmitBtn.disabled = false;
      elements.authSubmitBtn.textContent = originalBtnText;
    }
  }

  function loginSuccess(token, user) {
    state.token = token;
    state.user = user;
    localStorage.setItem('token', token);
    localStorage.setItem('user', JSON.stringify(user));
    elements.authForm.reset();
    loadTickets();
  }

  function signOut() {
    state.token = '';
    state.user = null;
    state.tickets = [];
    state.currentTicket = null;
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setAuthMode('login');
    showView(elements.authView);
  }

  // --- Tickets Management ---
  async function loadTickets() {
    showView(elements.ticketsView);
    closeNewTicketPanel();

    try {
      const data = await api('/tickets');
      state.tickets = data.tickets || [];
      renderTicketList();
    } catch (err) {
      if (state.token) {
        elements.ticketList.innerHTML = `<p class="form-error" style="padding: 16px;">Failed to load tickets: ${err.message}</p>`;
      }
    }
  }

  function renderTicketList() {
    elements.ticketList.innerHTML = '';

    if (state.tickets.length === 0) {
      elements.ticketList.classList.add('hidden');
      elements.emptyState.classList.remove('hidden');
      return;
    }

    elements.emptyState.classList.add('hidden');
    elements.ticketList.classList.remove('hidden');

    state.tickets.forEach((ticket) => {
      const row = document.createElement('div');
      row.className = `ticket-row ${ticket.status === 'closed' ? 'is-closed' : ''}`;
      row.setAttribute('role', 'button');
      row.setAttribute('tabindex', '0');
      row.setAttribute('aria-label', `${ticket.title}, status ${getReadableStatus(ticket.status)}`);

      row.innerHTML = `
        <div class="ticket-row-status">
          <span class="status-dot status-${ticket.status}"></span>
          <span class="status-label">${getReadableStatus(ticket.status)}</span>
        </div>
        <div class="ticket-row-title">${escapeHTML(ticket.title)}</div>
        <div class="ticket-row-time">${formatRelativeTime(ticket.created_at)}</div>
      `;

      row.addEventListener('click', () => openTicketDetail(ticket.id));
      row.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          openTicketDetail(ticket.id);
        }
      });

      elements.ticketList.appendChild(row);
    });
  }

  function openNewTicketPanel() {
    elements.newTicketPanel.classList.remove('collapsed');
    elements.newTicketPanel.setAttribute('aria-hidden', 'false');
    elements.ticketTitle.focus();
  }

  function closeNewTicketPanel() {
    elements.newTicketPanel.classList.add('collapsed');
    elements.newTicketPanel.setAttribute('aria-hidden', 'true');
    elements.newTicketForm.reset();
    elements.ticketTitleError.textContent = '';
    elements.newTicketGeneralError.textContent = '';
  }

  async function handleCreateTicketSubmit(e) {
    e.preventDefault();
    elements.ticketTitleError.textContent = '';
    elements.newTicketGeneralError.textContent = '';

    const title = elements.ticketTitle.value.trim();
    const description = elements.ticketDescription.value.trim() || null;

    if (!title) {
      elements.ticketTitleError.textContent = 'Title is required';
      elements.ticketTitle.focus();
      return;
    }

    elements.createTicketBtn.disabled = true;
    elements.createTicketBtn.textContent = 'Creating…';

    try {
      const created = await api('/tickets', {
        method: 'POST',
        body: JSON.stringify({ title, description }),
      });

      closeNewTicketPanel();
      // Prepend or reload
      state.tickets.unshift(created);
      renderTicketList();
    } catch (err) {
      elements.newTicketGeneralError.textContent = err.message;
    } finally {
      elements.createTicketBtn.disabled = false;
      elements.createTicketBtn.textContent = 'Create ticket';
    }
  }

  // --- Ticket Detail View ---
  async function openTicketDetail(ticketId) {
    try {
      const ticket = await api(`/tickets/${ticketId}`);
      state.currentTicket = ticket;
      renderTicketDetail();
      showView(elements.detailView);
    } catch (err) {
      alert(`Could not load ticket: ${err.message}`);
    }
  }

  function renderTicketDetail() {
    const ticket = state.currentTicket;
    if (!ticket) return;

    elements.detailTitle.textContent = ticket.title;
    elements.detailId.textContent = `#${ticket.id.slice(0, 8)}`;
    elements.detailTimestamp.textContent = `created ${formatRelativeTime(ticket.created_at)}`;

    // Status Indicator
    elements.detailStatusDot.className = `status-dot status-${ticket.status}`;
    elements.detailStatusText.textContent = getReadableStatus(ticket.status);

    // Description
    if (ticket.description && ticket.description.trim()) {
      elements.detailDescription.textContent = ticket.description;
      elements.detailDescription.style.color = 'var(--ink)';
    } else {
      elements.detailDescription.textContent = 'No description provided.';
      elements.detailDescription.style.color = 'var(--ink-soft)';
    }

    // Status Action Button
    // Per UI-SPEC.md:
    // Only one status action shown at a time — the single valid next step.
    // If open: "Mark in progress"
    // If in_progress: "Mark closed"
    // If closed: show no action, just quiet note "This ticket is closed."
    if (ticket.status === 'open') {
      elements.detailActionBtn.textContent = 'Mark in progress';
      elements.detailActionBtn.classList.remove('hidden');
      elements.detailActionBtn.disabled = false;
      elements.detailCloseDirectBtn.classList.remove('hidden');
      elements.detailCloseDirectBtn.disabled = false;
      elements.detailCloseDirectBtn.textContent = 'Close ticket';
      elements.detailClosedNotice.classList.add('hidden');
    } else if (ticket.status === 'in_progress') {
      elements.detailActionBtn.textContent = 'Mark closed';
      elements.detailActionBtn.classList.remove('hidden');
      elements.detailActionBtn.disabled = false;
      elements.detailCloseDirectBtn.classList.add('hidden');
      elements.detailClosedNotice.classList.add('hidden');
    } else {
      elements.detailActionBtn.classList.add('hidden');
      elements.detailCloseDirectBtn.classList.add('hidden');
      elements.detailClosedNotice.classList.remove('hidden');
    }
  }

  async function handleDetailStatusAction() {
    const ticket = state.currentTicket;
    if (!ticket) return;

    let targetStatus = '';
    if (ticket.status === 'open') {
      targetStatus = 'in_progress';
    } else if (ticket.status === 'in_progress') {
      targetStatus = 'closed';
    } else {
      return;
    }

    const originalText = elements.detailActionBtn.textContent;
    elements.detailActionBtn.disabled = true;
    elements.detailActionBtn.textContent = 'Updating…';

    try {
      const updated = await api(`/tickets/${ticket.id}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ status: targetStatus }),
      });

      // Update current ticket and list
      state.currentTicket = updated;
      const idx = state.tickets.findIndex((t) => t.id === updated.id);
      if (idx !== -1) {
        state.tickets[idx] = updated;
      }

      // Quiet confirmation hold per spec (~200ms)
      setTimeout(() => {
        renderTicketDetail();
      }, 200);
    } catch (err) {
      alert(`Failed to update status: ${err.message}`);
      elements.detailActionBtn.disabled = false;
      elements.detailActionBtn.textContent = originalText;
    }
  }

  async function handleDetailCloseDirectAction() {
    const ticket = state.currentTicket;
    if (!ticket || ticket.status !== 'open') return;

    elements.detailCloseDirectBtn.disabled = true;
    elements.detailActionBtn.disabled = true;
    elements.detailCloseDirectBtn.textContent = 'Closing…';

    try {
      const updated = await api(`/tickets/${ticket.id}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ status: 'closed' }),
      });

      state.currentTicket = updated;
      const idx = state.tickets.findIndex((t) => t.id === updated.id);
      if (idx !== -1) {
        state.tickets[idx] = updated;
      }

      setTimeout(() => {
        renderTicketDetail();
      }, 200);
    } catch (err) {
      alert(`Failed to close ticket: ${err.message}`);
      elements.detailCloseDirectBtn.disabled = false;
      elements.detailActionBtn.disabled = false;
      elements.detailCloseDirectBtn.textContent = 'Close ticket';
    }
  }

  function escapeHTML(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // --- Event Listeners ---
  function initEvents() {
    elements.themeToggle.addEventListener('click', toggleTheme);

    elements.brandLink.addEventListener('click', (e) => {
      e.preventDefault();
      if (state.token) {
        loadTickets();
      } else {
        showView(elements.authView);
      }
    });

    elements.authModeToggle.addEventListener('click', () => {
      setAuthMode(state.authMode === 'login' ? 'register' : 'login');
    });

    elements.authForm.addEventListener('submit', handleAuthSubmit);
    elements.signOutBtn.addEventListener('click', signOut);

    elements.openNewTicketBtn.addEventListener('click', openNewTicketPanel);
    elements.topNewTicketBtn.addEventListener('click', () => {
      showView(elements.ticketsView);
      openNewTicketPanel();
    });
    elements.emptyNewTicketBtn.addEventListener('click', openNewTicketPanel);
    elements.cancelNewTicketBtn.addEventListener('click', closeNewTicketPanel);
    elements.newTicketForm.addEventListener('submit', handleCreateTicketSubmit);

    elements.backToTicketsBtn.addEventListener('click', () => {
      showView(elements.ticketsView);
      renderTicketList();
    });

    elements.detailActionBtn.addEventListener('click', handleDetailStatusAction);
    elements.detailCloseDirectBtn.addEventListener('click', handleDetailCloseDirectAction);
  }

  // --- App Initialization ---
  function init() {
    updateThemeUI();
    initEvents();

    if (state.token) {
      loadTickets();
    } else {
      showView(elements.authView);
    }
  }

  init();
})();
