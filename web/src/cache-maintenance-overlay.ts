import { createApp } from 'vue';
import PrimeVue from 'primevue/config';
import ToastService from 'primevue/toastservice';
import CacheMaintenanceOverlay from './CacheMaintenanceOverlay.vue';

createApp(CacheMaintenanceOverlay)
  .use(PrimeVue, { unstyled: true })
  .use(ToastService)
  .mount('#cache-maintenance-overlay');
