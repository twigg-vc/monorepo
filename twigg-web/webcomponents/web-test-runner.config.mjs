import { esbuildPlugin } from '@web/dev-server-esbuild';
import { puppeteerLauncher } from '@web/test-runner-puppeteer';

export default {
    files: '*.test.ts',
    nodeResolve: true,
    browsers: [puppeteerLauncher()],
    plugins: [esbuildPlugin({ ts: true, target: 'es2020' })],
};
