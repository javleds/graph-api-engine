/// <reference types="vite/client" />

declare module '*.md?raw' { const value: string; export default value; }
declare module '*.json?raw' { const value: string; export default value; }
