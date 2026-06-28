# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: node-lifecycle.spec.ts >> Node Manager — full provision/cordon/delete lifecycle >> provision → verify → cordon → delete lifecycle
- Location: tests/node-lifecycle.spec.ts:87:7

# Error details

```
Error: Command failed: ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -i /home/claude/.ssh/id_ed25519 samcaldwell@192.168.3.159 sudo su -l samcaldwell -c 'cd ~/git/svr00 && vagrant up convocate07 --provision'
Warning: Permanently added '192.168.3.159' (ED25519) to the list of known hosts.
bash: warning: setlocale: LC_ALL: cannot change locale (en_US.UTF-8)
bash: warning: setlocale: LC_ALL: cannot change locale (en_US.UTF-8)
A Vagrant environment or target machine is required to run this
command. Run `vagrant init` to create a new Vagrant environment. Or,
get an ID of a target machine from `vagrant global-status` to run
this command on. A final option is to change to a directory with a
Vagrantfile and to try again.

```

# Test source

```ts
  1   | import { test, expect } from "@playwright/test";
  2   | import { execSync } from "child_process";
  3   | 
  4   | const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
  5   | const SVR00 = "192.168.3.159";
  6   | const SVR00_USER = "samcaldwell";
  7   | const SSH_KEY = process.env.SSH_KEY || `${process.env.HOME}/.ssh/id_ed25519`;
  8   | const NODE_IP = "192.168.56.17";
  9   | const NODE_NAME = "convocate07";
  10  | const NODE_USER = "convocate";
  11  | const NODE_PASS = "Elocin3125!";
  12  | const NODE_LOCATION = "test-machine";
  13  | 
  14  | // ---------------------------------------------------------------------------
  15  | // SSH helper: run a command on svr00 via SSH
  16  | // ---------------------------------------------------------------------------
  17  | function ssh(cmd: string, timeout = 60_000): string {
  18  |   const sshCmd = [
  19  |     "ssh",
  20  |     "-o", "StrictHostKeyChecking=no",
  21  |     "-o", "UserKnownHostsFile=/dev/null",
  22  |     "-o", "ConnectTimeout=10",
  23  |     "-i", SSH_KEY,
  24  |     `${SVR00_USER}@${SVR00}`,
  25  |     cmd,
  26  |   ].join(" ");
> 27  |   return execSync(sshCmd, {
      |                  ^ Error: Command failed: ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -i /home/claude/.ssh/id_ed25519 samcaldwell@192.168.3.159 sudo su -l samcaldwell -c 'cd ~/git/svr00 && vagrant up convocate07 --provision'
  28  |     timeout,
  29  |     encoding: "utf-8",
  30  |     stdio: ["pipe", "pipe", "pipe"],
  31  |   }).trim();
  32  | }
  33  | 
  34  | // ---------------------------------------------------------------------------
  35  | // Login helper
  36  | // ---------------------------------------------------------------------------
  37  | async function login(page: import("@playwright/test").Page) {
  38  |   await page.goto(APP);
  39  |   await page.waitForSelector("input", { timeout: 10000 });
  40  |   const inputs = page.locator("input");
  41  |   await inputs.nth(0).fill("admin");
  42  |   await inputs.nth(1).fill("test");
  43  |   await inputs.nth(2).fill("123456");
  44  |   await page.locator("button").filter({ hasText: /sign in/i }).click();
  45  |   await page.waitForSelector("[class*='dock'], [class*='unity-desktop']", {
  46  |     timeout: 10000,
  47  |   });
  48  | }
  49  | 
  50  | // ---------------------------------------------------------------------------
  51  | // Open Node Manager
  52  | // ---------------------------------------------------------------------------
  53  | async function openNodeManager(page: import("@playwright/test").Page) {
  54  |   const icon = page.locator("img[alt='Node Manager']");
  55  |   const box = await icon.boundingBox();
  56  |   if (box)
  57  |     await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  58  |   await page.waitForTimeout(3000);
  59  | }
  60  | 
  61  | // ---------------------------------------------------------------------------
  62  | // Test
  63  | // ---------------------------------------------------------------------------
  64  | test.describe("Node Manager — full provision/cordon/delete lifecycle", () => {
  65  |   // Increase overall test timeout: provisioning a real node takes minutes
  66  |   test.setTimeout(600_000); // 10 minutes
  67  | 
  68  |   // Cleanup: always destroy convocate07 on pass or fail
  69  |   test.afterAll(() => {
  70  |     try {
  71  |       // Remove from K8s if still present (ignore errors)
  72  |       try {
  73  |         ssh(`kubectl delete node ${NODE_NAME} --ignore-not-found=true`, 30_000);
  74  |       } catch {
  75  |         /* already gone */
  76  |       }
  77  |       // Destroy the vagrant VM
  78  |       ssh(
  79  |         `sudo su -l ${SVR00_USER} -c 'cd ~/git/svr00 && vagrant destroy ${NODE_NAME} --force'`,
  80  |         120_000
  81  |       );
  82  |     } catch (e) {
  83  |       console.error("cleanup failed:", e);
  84  |     }
  85  |   });
  86  | 
  87  |   test("provision → verify → cordon → delete lifecycle", async ({ page, request }) => {
  88  |     // ──────────────────────────────────────────────────────────────────────
  89  |     // Step 1: Provision the VM via Vagrant
  90  |     // ──────────────────────────────────────────────────────────────────────
  91  |     console.log("[step 1] provisioning convocate07 VM via vagrant...");
  92  |     ssh(
  93  |       `sudo su -l ${SVR00_USER} -c 'cd ~/git/svr00 && vagrant up ${NODE_NAME} --provision'`,
  94  |       300_000 // 5 minutes
  95  |     );
  96  | 
  97  |     // Verify the VM is running
  98  |     const vmStatus = ssh(
  99  |       `sudo su -l ${SVR00_USER} -c 'cd ~/git/svr00 && vagrant status ${NODE_NAME}' | grep ${NODE_NAME}`
  100 |     );
  101 |     expect(vmStatus).toContain("running");
  102 |     console.log("[step 1] VM is running");
  103 | 
  104 |     // ──────────────────────────────────────────────────────────────────────
  105 |     // Step 2: Verify convocate07 does NOT appear in Node Manager
  106 |     // ──────────────────────────────────────────────────────────────────────
  107 |     console.log("[step 2] verifying convocate07 is NOT in node list...");
  108 |     await login(page);
  109 |     await openNodeManager(page);
  110 | 
  111 |     let bodyText = await page.textContent("body");
  112 |     expect(bodyText).not.toContain(NODE_NAME);
  113 |     console.log("[step 2] confirmed: convocate07 not in node list");
  114 | 
  115 |     // ──────────────────────────────────────────────────────────────────────
  116 |     // Step 3: Use UI to provision the node
  117 |     // ──────────────────────────────────────────────────────────────────────
  118 |     console.log("[step 3] clicking Provision Node button...");
  119 |     await page.locator("button").filter({ hasText: /Provision Node/i }).click();
  120 |     await page.waitForTimeout(1000);
  121 | 
  122 |     // Fill in the provision form
  123 |     // Find the modal inputs by their labels
  124 |     const hostInput = page.locator("input").filter({ has: page.locator("..").locator("text=Host") }).first();
  125 |     const userInput = page.locator("input").filter({ has: page.locator("..").locator("text=SSH User") }).first();
  126 |     const passInput = page.locator("input[type='password']").last();
  127 |     const locationInput = page.locator("input").filter({ has: page.locator("..").locator("text=Location") }).first();
```