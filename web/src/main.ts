import { createApp } from 'vue';
import PrimeVue from 'primevue/config';
import App from './App.vue';
import { i18n } from './i18n';
createApp(App).use(PrimeVue, { unstyled: true, license: import.meta.env.VITE_PRIMEUI_LICENSE_KEY }).use(i18n).mount('#app');
