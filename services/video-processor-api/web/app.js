const state = { token: null, identity: null, timer: null, mode: "login", refreshedAt: null, stale: false, downloading: new Set(), snapshot: null };

// Espelha MAX_UPLOAD_BYTES e supportedExtensions da API; a API continua validando.
const MAX_UPLOAD_BYTES = 500 * 1024 * 1024;
const EXTENSIONS = [".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm"];

const STATUS = { PENDING: "Na fila", PROCESSING: "Processando", COMPLETED: "Concluído", FAILED: "Falhou" };

const MESSAGES = {
  invalid_credentials: "E-mail ou senha incorretos.",
  email_taken: "Este e-mail já tem conta.",
  weak_password: "A senha precisa ter pelo menos 8 caracteres.",
  invalid_email: "Este e-mail não é válido.",
  invalid_body: "Preencha e-mail e senha.",
  unsupported_format: "Formato não suportado. Envie mp4, avi, mov, mkv, wmv, flv ou webm.",
  video_too_large: "O vídeo passa de 500 MB.",
  upload_not_found: "O arquivo ainda não chegou ao armazenamento. Tente de novo em instantes.",
  upload_mismatch: "O arquivo enviado não confere com este vídeo. Envie de novo.",
  not_completed: "O processamento ainda não terminou.",
  not_found: "Vídeo não encontrado.",
  forbidden: "Este vídeo não é seu.",
  internal_error: "O servidor falhou. Tente de novo em instantes.",
  offline: "Sem resposta do servidor. Confira a conexão e tente de novo.",
};

const MODES = {
  login: { title: "Entrar", hint: "Envie vídeos e baixe os frames em um zip.", password: "Senha", autocomplete: "current-password", label: "Entrar", busy: "Entrando…" },
  signup: { title: "Criar conta", hint: "Você entra direto depois do cadastro.", password: "Senha — pelo menos 8 caracteres", autocomplete: "new-password", label: "Criar conta", busy: "Criando conta…" },
};

const $ = (id) => document.getElementById(id);

const esc = (value) => String(value ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

class ApiError extends Error {
  constructor(status, code, message) {
    super(MESSAGES[code] || message || MESSAGES.internal_error);
    this.status = status;
    this.code = code;
  }
}

function decodeJWT(token) {
  const payload = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
  return JSON.parse(atob(payload));
}

function identityHeaders() {
  return {
    Authorization: `Bearer ${state.token}`,
    "X-User-Id": state.identity.sub,
    "X-User-Email": state.identity.email,
  };
}

async function request(path, options = {}, headers = {}) {
  let res;
  try {
    res = await fetch(path, { ...options, headers: { "Content-Type": "application/json", ...headers, ...(options.headers || {}) } });
  } catch {
    throw new ApiError(0, "offline");
  }
  const body = res.status === 204 ? null : await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, body?.error, body?.message);
  return body;
}

const api = (path, options) => request(path, options, identityHeaders());

// Trava um formulário inteiro enquanto uma requisição está em voo: campos, botões e abas.
function lock(form, busy) {
  form.setAttribute("aria-busy", String(busy));
  form.querySelectorAll("input, button").forEach((el) => { el.disabled = busy; });
  form.querySelectorAll("[data-label]").forEach((btn) => { btn.textContent = busy ? btn.dataset.busy : btn.dataset.label; });
}

function feedback(id, text, error = false) {
  const el = $(id);
  el.className = error ? "feedback error" : "feedback";
  el.textContent = text;
  return el;
}

// Sessão

function setSession(token) {
  const identity = decodeJWT(token);
  if (identity.exp && identity.exp * 1000 < Date.now()) throw new Error("expired");
  state.token = token;
  state.identity = identity;
  try { localStorage.setItem("token", token); } catch {}
  $("who").textContent = identity.email;
  $("session").hidden = false;
  $("auth").hidden = true;
  $("app").hidden = false;
  refresh();
  state.timer = setInterval(refresh, 5000);
}

function clearSession(notice) {
  clearInterval(state.timer);
  state.token = null;
  state.identity = null;
  state.refreshedAt = null;
  state.snapshot = null;
  feedback("upload-feedback", "");
  $("empty").hidden = true;
  try { localStorage.removeItem("token"); } catch {}
  $("session").hidden = true;
  $("auth").hidden = false;
  $("app").hidden = true;
  setMode("login");
  feedback("auth-feedback", notice || "");
}

// Entrar / criar conta

function setMode(mode) {
  state.mode = mode;
  const m = MODES[mode];
  const form = $("auth-form");
  form.querySelectorAll("[role=tab]").forEach((tab) => tab.setAttribute("aria-selected", String(tab.dataset.mode === mode)));
  $("auth-title").textContent = m.title;
  $("auth-hint").textContent = m.hint;
  $("password-label").textContent = m.password;
  form.elements.password.autocomplete = m.autocomplete;
  const submit = $("auth-submit");
  submit.dataset.label = m.label;
  submit.dataset.busy = m.busy;
  submit.textContent = m.label;
  feedback("auth-feedback", "");
}

$("auth-form").querySelectorAll("[role=tab]").forEach((tab) => tab.addEventListener("click", () => setMode(tab.dataset.mode)));

// Oferece o outro modo quando o erro é "conta não existe" ou "conta já existe".
function offerMode(text, mode, label) {
  const el = feedback("auth-feedback", `${text} `, true);
  const link = document.createElement("button");
  link.type = "button";
  link.className = "link";
  link.textContent = label;
  link.addEventListener("click", () => setMode(mode));
  el.append(link);
}

function validateAuth(form) {
  const { email, password } = form.elements;
  if (!email.value || !email.validity.valid) return "Informe um e-mail válido.";
  if (!password.value) return "Informe a senha.";
  if (state.mode === "signup" && password.value.length < 8) return MESSAGES.weak_password;
  return null;
}

$("auth-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  const invalid = validateAuth(form);
  if (invalid) { feedback("auth-feedback", invalid, true); return; }

  const credentials = JSON.stringify({ email: form.elements.email.value.trim(), password: form.elements.password.value });
  lock(form, true);
  $("auth-busy").hidden = false;
  feedback("auth-feedback", "");
  try {
    if (state.mode === "signup") await request("/auth/v1/signup", { method: "POST", body: credentials });
    const { token } = await request("/auth/v1/login", { method: "POST", body: credentials });
    form.reset();
    setSession(token);
  } catch (err) {
    if (err.code === "invalid_credentials") offerMode(err.message + " Ainda não tem conta?", "signup", "Criar conta com este e-mail");
    else if (err.code === "email_taken") offerMode(err.message, "login", "Entrar com ele");
    else feedback("auth-feedback", err.message, true);
  } finally {
    lock(form, false);
    $("auth-busy").hidden = true;
  }
});

$("logout").addEventListener("click", () => clearSession());

// Envio

const formatSize = (bytes) => (bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.ceil(bytes / 1024)} KB`);

function validateFile(file) {
  const ext = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
  if (!EXTENSIONS.includes(ext)) return MESSAGES.unsupported_format;
  if (file.size > MAX_UPLOAD_BYTES) return `${MESSAGES.video_too_large} Este tem ${formatSize(file.size)}.`;
  return null;
}

// Escolher o arquivo já inicia o envio: valida no navegador e chama send.
function pick(file) {
  feedback("upload-feedback", "");
  $("file").value = "";
  if (!file) return;
  const invalid = validateFile(file);
  if (invalid) { feedback("upload-feedback", invalid, true); return; }
  send(file);
}

$("file").addEventListener("change", (e) => pick(e.target.files[0]));

const dropzone = $("upload-form");
dropzone.querySelector(".idle").addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); $("file").click(); } });
dropzone.addEventListener("submit", (e) => e.preventDefault());
["dragenter", "dragover"].forEach((type) => dropzone.addEventListener(type, (e) => {
  if (dropzone.dataset.state === "sending") return;
  e.preventDefault();
  dropzone.classList.add("over");
}));
["dragleave", "drop"].forEach((type) => dropzone.addEventListener(type, () => dropzone.classList.remove("over")));
dropzone.addEventListener("drop", (e) => {
  if (dropzone.dataset.state === "sending") return;
  e.preventDefault();
  pick(e.dataTransfer.files[0]);
});

function putWithProgress(url, headers, file) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    for (const [name, value] of Object.entries(headers)) xhr.setRequestHeader(name, value);
    xhr.upload.onprogress = (ev) => {
      if (!ev.lengthComputable) return;
      const pct = Math.round((ev.loaded / ev.total) * 100);
      $("sending-bar").style.width = `${pct}%`;
      $("sending-step").textContent = `Enviando ao armazenamento… ${pct}%`;
    };
    xhr.onload = () => (xhr.status < 300 ? resolve() : reject(new Error(`O armazenamento recusou o arquivo (${xhr.status}).`)));
    xhr.onerror = () => reject(new Error("O envio ao armazenamento falhou. Tente de novo."));
    xhr.send(file);
  });
}

async function send(file) {
  const form = $("upload-form");
  form.dataset.state = "sending";
  form.setAttribute("aria-busy", "true");
  $("logout").disabled = true;
  $("sending-name").textContent = file.name;
  $("sending-bar").style.width = "0";
  try {
    $("sending-step").textContent = "Reservando envio…";
    const ticket = await api("/v1/videos", {
      method: "POST",
      body: JSON.stringify({ filename: file.name, contentType: file.type || "application/octet-stream", sizeBytes: file.size }),
    });
    $("sending-step").textContent = "Enviando ao armazenamento… 0%";
    await putWithProgress(ticket.uploadUrl, ticket.uploadHeaders, file);
    $("sending-step").textContent = "Confirmando…";
    await api(`/v1/videos/${ticket.videoId}/process`, { method: "POST" });
    feedback("upload-feedback", `${file.name} entrou na fila. Você recebe um e-mail quando terminar.`);
    refresh();
  } catch (err) {
    feedback("upload-feedback", `${file.name}: ${err.message}`, true);
  } finally {
    form.dataset.state = "idle";
    form.setAttribute("aria-busy", "false");
    $("logout").disabled = false;
  }
}

// Lista

async function download(videoId, btn) {
  state.downloading.add(videoId);
  btn.disabled = true;
  btn.textContent = "Preparando…";
  try {
    const { downloadUrl } = await api(`/v1/videos/${videoId}/download`);
    window.location.assign(downloadUrl);
  } catch (err) {
    feedback("upload-feedback", err.message, true);
  } finally {
    state.downloading.delete(videoId);
    // O polling pode ter redesenhado a grade no meio; procura o botão de novo em vez de confiar na referência.
    const current = $("grid").querySelector(`[data-download="${CSS.escape(videoId)}"]`) || btn;
    current.disabled = false;
    current.textContent = "Baixar zip";
  }
}

function reasonText(reason, attempts) {
  if (reason.startsWith("unprocessable video")) return "Arquivo inválido: o vídeo não pôde ser lido";
  if (reason.startsWith("video object not found")) return "Arquivo não encontrado no armazenamento";
  return `Falhou após ${attempts} ${attempts === 1 ? "tentativa" : "tentativas"}`;
}

const attemptsText = (n) => (n === 0 ? "aguardando o worker" : n === 1 ? "1 tentativa" : `${n}ª tentativa`);
const dateText = (iso) => new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });

function cell(it) {
  const failed = it.status === "FAILED";
  const done = it.status === "COMPLETED";
  const downloading = state.downloading.has(it.videoId);
  const summary = done
    ? `${it.result?.frameCount ?? 0} frames · ${attemptsText(it.attempts)}`
    : failed
      ? `<span class="reason" title="${esc(it.result?.failureReason)}">${esc(reasonText(it.result?.failureReason || "", it.attempts))}</span>`
      : attemptsText(it.attempts);
  return `
    <div class="frame cell">
      <span class="name">${esc(it.filename)}</span>
      <span class="meta">${summary}<br>${dateText(it.createdAt)}</span>
      ${it.status === "PROCESSING" ? '<div class="bar sweep"><i></i></div>' : ""}
      <div class="foot">
        <span class="status status-${esc(it.status)}">${STATUS[it.status] || esc(it.status)}</span>
        ${done ? `<button type="button" class="dl" data-download="${esc(it.videoId)}" ${downloading ? "disabled" : ""}>${downloading ? "Preparando…" : "Baixar zip"}</button>` : ""}
      </div>
    </div>`;
}

// Só mexe no DOM quando a lista mudou, para não interromper um clique ou uma seleção no meio do polling.
function render(items) {
  const snapshot = JSON.stringify(items);
  if (snapshot === state.snapshot) return;
  state.snapshot = snapshot;
  const grid = $("grid");
  grid.querySelectorAll(".cell:not(.new)").forEach((el) => el.remove());
  grid.insertAdjacentHTML("beforeend", items.map(cell).join(""));
  grid.querySelectorAll("[data-download]").forEach((btn) => btn.addEventListener("click", () => download(btn.dataset.download, btn)));
  $("empty").hidden = items.length > 0;
}

async function refresh() {
  if (!state.token) return;
  try {
    const { items } = await api("/v1/videos");
    render(items);
    state.refreshedAt = Date.now();
    state.stale = false;
  } catch (err) {
    if (err.status === 401) { clearSession("Sua sessão expirou. Entre de novo."); return; }
    state.stale = true;
  }
  tickFreshness();
}

function tickFreshness() {
  if (!state.refreshedAt) { $("freshness").textContent = ""; return; }
  const secs = Math.max(0, Math.round((Date.now() - state.refreshedAt) / 1000));
  $("freshness").textContent = state.stale ? `sem resposta há ${secs} s` : `atualizado há ${secs} s`;
}
setInterval(tickFreshness, 1000);

// Boot

setMode("login");
let saved = null;
try { saved = localStorage.getItem("token"); } catch {}
if (saved) {
  try { setSession(saved); } catch { clearSession("Sua sessão expirou. Entre de novo."); }
}
