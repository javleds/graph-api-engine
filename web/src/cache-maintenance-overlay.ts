import { createApp } from 'vue';
import PrimeVue from 'primevue/config';
import ToastService from 'primevue/toastservice';
import CacheMaintenanceOverlay from './CacheMaintenanceOverlay.vue';

createApp(CacheMaintenanceOverlay)
  .use(PrimeVue, { unstyled: true, license: import.meta.env.VITE_PRIMEUI_LICENSE_KEY })
  .use(ToastService)
  .mount('#cache-maintenance-overlay');
