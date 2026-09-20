<script setup lang="ts">
import { computed, ref } from 'vue';
import MarkdownIt from 'markdown-it';
import v1Guide from '../../docs/result-delivery-v1.md?raw';
import v2Guide from '../../docs/result-delivery-v2.md?raw';
import { messagesFor, uiLanguage } from './i18n';

const version = ref<'v1' | 'v2'>('v1');
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true });
const language = uiLanguage();
const ui = messagesFor(language);
const content = computed(() => markdown.render(version.value === 'v1' ? v1Guide : v2Guide));
</script>

<template>
  <section class="result-delivery">
    <div class="result-delivery-intro">
      <h2>{{ ui.results }}</h2>
      <p>{{ language === 'en' ? 'V1 delivers the graph when execution finishes. V2 delivers each complete chain as it finishes: a single object with its children, or each collection item with all descendants. Both versions remain available through the API; a webhook changes which notifications you receive and when.' : 'V1 entrega el grafo al finalizar la ejecución. V2 entrega cada cadena completa a medida que termina: un objeto single con sus hijos, o cada elemento de una colección con todos sus descendientes. En ambas versiones puedes consultar por API; si incluyes un webhook, cambia qué notificaciones recibirás y cuándo.' }}</p>
    </div>
    <fieldset class="version-selector">
      <legend>{{ language === 'en' ? 'API version' : 'Versión de la API' }}</legend>
      <label :class="{ selected: version === 'v1' }"><input v-model="version" type="radio" name="result-version" value="v1" aria-controls="result-version-content">V1 · {{ language === 'en' ? 'Final result' : 'Resultado final' }}</label>
      <label :class="{ selected: version === 'v2' }"><input v-model="version" type="radio" name="result-version" value="v2" aria-controls="result-version-content">V2 · {{ language === 'en' ? 'Complete chains' : 'Cadenas completas' }}</label>
    </fieldset>
    <article id="result-version-content" class="markdown result-delivery-content" :aria-label="`Obtención de resultados ${version.toUpperCase()}`" v-html="content"></article>
  </section>
</template>

<style scoped>
.result-delivery { display: grid; gap: 22px; }
.result-delivery-intro { max-width: 900px; }
.result-delivery-intro p { margin-bottom: 0; }
.version-selector { display: flex; flex-wrap: wrap; gap: 12px; margin: 0; padding: 18px; border: 1px solid #263653; background: #101a30; }
.version-selector legend { padding: 0 8px; color: #c4cce0; font-size: 14px; }
.version-selector label { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border: 1px solid #263653; border-radius: 3px; cursor: pointer; }
.version-selector label.selected { border-color: #4cdcb3; color: #4cdcb3; background: #102d30; }
.version-selector label:focus-within { outline: 2px solid #68e9ff; outline-offset: 3px; }
.version-selector input { width: auto; margin: 0; accent-color: #4cdcb3; }
.result-delivery-content :deep(h2), .result-delivery-content :deep(h3) { margin: 28px 0 12px; letter-spacing: -.5px; }
.result-delivery-content :deep(p), .result-delivery-content :deep(li) { line-height: 1.7; }
.result-delivery-content :deep(table) { display: block; overflow-x: auto; width: 100%; border-collapse: collapse; margin: 20px 0; }
.result-delivery-content :deep(th), .result-delivery-content :deep(td) { padding: 12px; border: 1px solid #263653; text-align: left; vertical-align: top; min-width: 160px; line-height: 1.6; }
.result-delivery-content :deep(th) { background: #121e36; }
.result-delivery-content :deep(code) { overflow-wrap: anywhere; }
.result-delivery-content :deep(pre code) { overflow-wrap: normal; }
@media (max-width: 760px) { .version-selector { flex-direction: column; } .version-selector label { padding: 12px; } }
</style>
