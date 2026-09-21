<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import Button from 'primevue/button';
import Dialog from 'primevue/dialog';
import Toast from 'primevue/toast';
import { useToast } from 'primevue/usetoast';

type CacheClearRequest = CustomEvent<{ clear: () => Promise<number> }>;
type Translation = {
  title: string;
  message: string;
  cancel: string;
  confirm: string;
  clearing: string;
  cleared: (count: number) => string;
  failed: string;
};

const translations: Record<'es' | 'en', Translation> = {
  es: {
    title: 'Limpiar caché',
    message: '¿Limpiar todas las respuestas almacenadas en caché? Esta acción afecta a todos los usuarios del servicio.',
    cancel: 'Cancelar',
    confirm: 'Limpiar caché',
    clearing: 'Limpiando caché…',
    cleared: (count) => `Caché limpiada: ${count} entradas eliminadas.`,
    failed: 'No se pudo limpiar la caché',
  },
  en: {
    title: 'Clear cache',
    message: 'Clear all stored response cache entries? This action affects every service user.',
    cancel: 'Cancel',
    confirm: 'Clear cache',
    clearing: 'Clearing cache…',
    cleared: (count) => `Cache cleared: ${count} entries deleted.`,
    failed: 'Could not clear the cache',
  },
};

const language = document.documentElement.lang === 'en' ? 'en' : 'es';
const copy = translations[language];
const toast = useToast();
const visible = ref(false);
const clearing = ref(false);
let clear: (() => Promise<number>) | null = null;

function open(event: Event) {
  clear = (event as CacheClearRequest).detail.clear;
  visible.value = true;
}

function cancel() {
  if (clearing.value) return;
  visible.value = false;
  clear = null;
}

async function confirm() {
  if (!clear || clearing.value) return;

  clearing.value = true;
  try {
    const deletedEntries = await clear();
    visible.value = false;
    toast.add({ severity: 'success', summary: copy.title, detail: copy.cleared(deletedEntries), life: 5000 });
  } catch (error) {
    const detail = error instanceof Error ? error.message : copy.failed;
    toast.add({ severity: 'error', summary: copy.failed, detail, life: 7000 });
  } finally {
    clearing.value = false;
    clear = null;
  }
}

onMounted(() => {
  window.addEventListener('playground:clear-cache', open);
  document.documentElement.dataset.cacheMaintenanceReady = 'true';
  window.dispatchEvent(new Event('playground:cache-maintenance-ready'));
});
onBeforeUnmount(() => window.removeEventListener('playground:clear-cache', open));
</script>

<template>
  <Toast position="top-right" />
  <Dialog
    v-model:visible="visible"
    modal
    :closable="false"
    :closeOnEscape="false"
    :dismissableMask="false"
    :header="copy.title"
    class="cache-clear-dialog"
  >
    <p class="cache-clear-dialog__message">{{ copy.message }}</p>
    <template #footer>
      <div class="cache-clear-dialog__actions">
        <Button type="button" class="cache-clear-dialog__cancel" :disabled="clearing" @click="cancel">
          {{ copy.cancel }}
        </Button>
        <Button type="button" class="cache-clear-dialog__confirm" :disabled="clearing" @click="confirm">
          {{ clearing ? copy.clearing : copy.confirm }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>

<style>
.p-dialog-mask,
[data-pc-section='mask'][data-p~='modal'] {
  align-items: center;
  background: rgb(5 12 25 / 72%);
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 1rem;
  position: fixed;
  z-index: 1200;
}

.cache-clear-dialog {
  background: #101b32;
  border: 1px solid #345078;
  border-radius: 0.75rem;
  box-shadow: 0 1.5rem 4rem rgb(0 0 0 / 45%);
  color: #e8f0ff;
  max-width: 32rem;
  width: 100%;
}

.cache-clear-dialog .p-dialog-header,
.cache-clear-dialog .p-dialog-content,
.cache-clear-dialog .p-dialog-footer {
  padding: 1.25rem;
}

.cache-clear-dialog .p-dialog-header {
  border-bottom: 1px solid #29415f;
  font-weight: 700;
}

.cache-clear-dialog .p-dialog-footer {
  border-top: 1px solid #29415f;
}

.cache-clear-dialog__message {
  color: #c7d6eb;
  line-height: 1.55;
  margin: 0;
}

.cache-clear-dialog__actions {
  display: flex;
  gap: 0.75rem;
  justify-content: flex-end;
}

.cache-clear-dialog__actions button {
  border-radius: 0.4rem;
  cursor: pointer;
  font: inherit;
  padding: 0.6rem 0.9rem;
}

.cache-clear-dialog__cancel {
  background: transparent;
  border: 1px solid #527197;
  color: #e8f0ff;
}

.cache-clear-dialog__confirm {
  background: #b63842;
  border: 1px solid #dc6570;
  color: #fff;
}

.cache-clear-dialog__actions button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.p-toast,
[data-pc-section='root'][data-p~='top-right'] {
  max-width: min(24rem, calc(100vw - 2rem));
  z-index: 1300;
}

.p-toast-message,
[data-pc-section='message'][role='alert'] {
  background: #101b32;
  border: 1px solid #527197;
  border-left-width: 0.3rem;
  border-radius: 0.6rem;
  box-shadow: 0 1rem 2rem rgb(0 0 0 / 35%);
  color: #e8f0ff;
  padding: 0.9rem 1rem;
  position: relative;
}

.p-toast-message-content,
[data-pc-section='messagecontent'] {
  align-items: flex-start;
  display: flex;
  gap: 0.6rem;
}

.p-toast-message-success,
[data-pc-section='message'][data-p~='success'] { border-left-color: #45ba8b; }
.p-toast-message-error,
[data-pc-section='message'][data-p~='error'] { border-left-color: #dc6570; }
.p-toast-summary,
[data-pc-section='summary'] { display: block; font-weight: 700; }
.p-toast-detail,
[data-pc-section='detail'] { color: #c7d6eb; display: block; margin-top: 0.25rem; }
.p-toast-close-button,
[data-pc-section='closebutton'] { background: transparent; border: 0; color: inherit; cursor: pointer; }
</style>
