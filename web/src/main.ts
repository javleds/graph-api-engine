import { createApp } from 'vue';
import PrimeVue from 'primevue/config';
import App from './App.vue';
import { i18n } from './i18n';
createApp(App).use(PrimeVue, { unstyled: true }).use(i18n).mount('#app');
