// lgtm[js/disabling-certificate-validation] -- PDV tests run against K8s clusters with self-signed TLS certificates
process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  timeout: 60000,
  retries: 1,
  reporter: [["list"]],
  projects: [
    { name: "pdv", testMatch: /pdv\.spec\.ts/ },
    { name: "smoke", testMatch: /smoke\.spec\.ts/ },
  ],
});
