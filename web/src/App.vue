<script setup lang="ts">
import { onMounted, ref } from 'vue';

const path = ref(location.pathname);
const username = ref('');
const password = ref('');
const error = ref('');
const loading = ref(false);

async function login() {
  loading.value = true; error.value = '';
  try {
    const response = await fetch('/playground/api/session', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: username.value, password: password.value }) });
    if (response.ok) { location.assign('/playground'); return; }
    error.value = response.status === 401 ? 'Credenciales incorrectas.' : 'No se pudo iniciar sesión. Intenta nuevamente.';
  } catch { error.value = 'No se pudo conectar con el servicio.'; }
  finally { loading.value = false; }
}
async function logout() { await fetch('/playground/api/session', { method: 'DELETE' }); location.assign('/login'); }
const pokemon = `{
  "definition": {
    "version": 1,
    "api": { "baseUrl": "https://pokeapi.co/api/v2", "limits": { "requests": 5, "per": "1s", "concurrency": 2 } },
    "calls": { "name": "pokemon", "type": "single", "select": "", "request": { "method": "GET", "path": "/pokemon/ditto" } }
  }
}`;
function tryExample() { location.assign('/playground?example=pokemon'); }
onMounted(async () => { if (path.value !== '/login') { const response = await fetch('/playground/api/session'); if (!response.ok) location.replace('/login'); } });
</script>

<template>
  <main class="shell">
    <header><a href="/playground" class="brand">↳ API ENGINE <small>v2</small></a><nav v-if="path !== '/login'"><a href="/playground">Playground</a><a href="/docs">Docs</a><a href="/examples">Ejemplos</a><button @click="logout">Cerrar sesión</button></nav></header>
    <section v-if="path === '/login'" class="login"><p class="eyebrow">CONSOLa DE EJECUCIÓN</p><h1>Inicia sesión.</h1><p>Accede al playground y a la documentación del motor.</p><form @submit.prevent="login"><label>Usuario<input v-model="username" autocomplete="username" required /></label><label>Contraseña<input v-model="password" type="password" autocomplete="current-password" required /></label><p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="loading">{{ loading ? 'Validando…' : 'Entrar al playground' }}</button></form></section>
    <section v-else-if="path === '/docs'" class="content"><p class="eyebrow">DOCUMENTACIÓN</p><h1>Opera y consume API Engine.</h1><h2>Servicio y arranque</h2><p>Configura <code>.env</code>, revisa <code>config/config.yml</code> y ejecuta <code>docker compose up -d --build</code>. <code>APP_URL</code> y <code>APP_PORT</code> definen la URL pública y el puerto publicado.</p><h2>Consumo de grafos</h2><p>Una definición tiene un proveedor común y uno o varios nodos raíz. Los nodos <code>single</code> representan recursos y <code>collection</code> representan listas; <code>children</code> enriquece cada elemento con relaciones dependientes.</p><p>Consulta la guía completa en el repositorio: <code>docs/api-request-guide.md</code> y <code>docs/api-v2-guide.md</code>.</p><h2>Destinos permitidos</h2><p>Usa URLs exactas, <code>https://*.example.com</code> para subdominios o <code>*</code> para hosts públicos. HTTP y redes privadas siguen sujetos a la configuración del operador.</p></section>
    <section v-else-if="path === '/examples'" class="content"><p class="eyebrow">EJEMPLOS</p><h1>PokéAPI</h1><p>Consulta Ditto desde PokéAPI y úsalo como base para explorar nodos y resultados.</p><pre>{{ pokemon }}</pre><button @click="tryExample">Inténtalo en playground →</button></section>
    <section v-else class="content"><h1>Abriendo playground…</h1></section>
  </main>
</template>

<style>
:root { color-scheme:dark; font-family:Inter,system-ui,sans-serif; background:#080f20; color:#e4ebff; } *{box-sizing:border-box} body{margin:0}.shell{min-height:100vh;background:radial-gradient(circle at 85% -20%,#16325a77,transparent 34rem),#080f20}header{height:58px;padding:0 max(18px,calc((100vw - 1280px)/2));display:flex;align-items:center;justify-content:space-between;background:#121b34;border-bottom:1px solid #1a2945}a{color:#c8d2e7;text-decoration:none}.brand{font-weight:800;letter-spacing:1.6px;color:#e4ebff}.brand small{padding:4px 6px;color:#85bff1;background:#1d2944;margin-left:8px}nav{display:flex;align-items:center;gap:18px}button{border:0;border-radius:3px;background:#4cdcb3;color:#071124;padding:11px 15px;font-weight:700;cursor:pointer}nav button{padding:7px 10px;background:#1d2944;color:#e4ebff}.content,.login{max-width:820px;margin:0 auto;padding:72px 24px}.login{max-width:440px}.eyebrow{font:700 11px ui-monospace,monospace;letter-spacing:1px;color:#4cdcb3}h1{font-size:clamp(32px,5vw,50px);letter-spacing:-2px;margin:8px 0 10px}h2{margin-top:36px}p{line-height:1.65;color:#c4cce0}form{margin-top:30px;display:grid;gap:16px;padding:24px;background:#101a30;border:1px solid #263653}label{display:grid;gap:7px;color:#c8d2e7}input{border:1px solid #263653;background:#071124;color:#e4ebff;padding:12px;border-radius:3px}.error{color:#ff9a8b;margin:0}pre{overflow:auto;background:#071124;border:1px solid #263653;padding:16px;color:#68e9ff}@media(max-width:600px){header{padding:0 14px}.brand{font-size:12px}nav{gap:10px;font-size:12px}}
</style>
