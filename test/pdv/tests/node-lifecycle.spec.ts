import { test, expect } from "@playwright/test";
import { execFileSync } from "child_process";

const APP = process.env.APP_URL || "https://app.convocate.asymmetric-effort.com";
const SVR00 = "192.168.3.159";
const SVR00_USER = "samcaldwell";
const SSH_KEY = process.env.SSH_KEY || `${process.env.HOME}/.ssh/id_ed25519`;
const NODE_IP = "192.168.56.17";
const NODE_NAME = "convocate07";
const NODE_USER = "convocate";
const NODE_PASS = "Elocin3125!";
const NODE_LOCATION = "test-machine";

// ---------------------------------------------------------------------------
// SSH helper: run a command on svr00 via SSH
// ---------------------------------------------------------------------------
function ssh(cmd: string, timeout = 60_000): string {
  return execFileSync("ssh", [
    "-o", "StrictHostKeyChecking=no",
    "-o", "UserKnownHostsFile=/dev/null",
    "-o", "ConnectTimeout=10",
    "-i", SSH_KEY,
    `${SVR00_USER}@${SVR00}`,
    cmd,
  ], {
    timeout,
    encoding: "utf-8",
    stdio: ["pipe", "pipe", "pipe"],
  }).trim();
}

// ---------------------------------------------------------------------------
// Login helper
// ---------------------------------------------------------------------------
async function login(page: import("@playwright/test").Page) {
  await page.goto(APP);
  // If already logged in (token in localStorage), the desktop renders directly
  try {
    await page.waitForSelector("input", { timeout: 5000 });
    const inputs = page.locator("input");
    await inputs.nth(0).fill("admin");
    await inputs.nth(1).fill("test");
    await inputs.nth(2).fill("123456");
    await page.locator("button").filter({ hasText: /sign in/i }).click();
  } catch {
    // Already logged in — desktop is visible
  }
  await page.waitForSelector("[class*='dock'], [class*='unity-desktop']", {
    timeout: 10000,
  });
}

// ---------------------------------------------------------------------------
// Open Node Manager
// ---------------------------------------------------------------------------
async function openNodeManager(page: import("@playwright/test").Page) {
  const icon = page.locator("img[alt='Node Manager']");
  const box = await icon.boundingBox();
  if (box)
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await page.waitForTimeout(3000);
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------
test.describe("Node Manager — full provision/cordon/delete lifecycle", () => {
  // Increase overall test timeout: provisioning a real node takes minutes
  test.setTimeout(600_000); // 10 minutes

  // Cleanup: always destroy convocate07 on pass or fail
  test.afterAll(() => {
    try {
      // Remove from K8s if still present (ignore errors)
      try {
        ssh(`kubectl delete node ${NODE_NAME} --ignore-not-found=true`, 30_000);
      } catch {
        /* already gone */
      }
      // Destroy the vagrant VM
      ssh(
        `cd /home/${SVR00_USER}/git/svr00 && vagrant destroy ${NODE_NAME} --force`,
        120_000
      );
    } catch (e) {
      console.error("cleanup failed:", e);
    }
  });

  test("provision → verify → cordon → delete lifecycle", async ({ page, request }) => {
    // ──────────────────────────────────────────────────────────────────────
    // Step 1: Provision the VM via Vagrant
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 1] provisioning convocate07 VM via vagrant...");
    ssh(
      `cd /home/${SVR00_USER}/git/svr00 && vagrant up ${NODE_NAME} --provision`,
      300_000 // 5 minutes
    );

    // Verify the VM is running
    const vmStatus = ssh(
      `cd /home/${SVR00_USER}/git/svr00 && vagrant status ${NODE_NAME} | grep ${NODE_NAME}`
    );
    expect(vmStatus).toContain("running");
    console.log("[step 1] VM is running");

    // Enable sudo for the convocate user via vagrant user (has sudo)
    console.log("[step 1b] enabling sudo for convocate user...");
    ssh(
      `cd /home/${SVR00_USER}/git/svr00 && vagrant ssh ${NODE_NAME} -c "sudo sh -c 'echo \\\"convocate ALL=(ALL) NOPASSWD:ALL\\\" > /etc/sudoers.d/convocate && chmod 0440 /etc/sudoers.d/convocate'"`,
      30_000
    );
    console.log("[step 1b] sudo configured");

    // ──────────────────────────────────────────────────────────────────────
    // Step 2: Verify convocate07 does NOT appear in Node Manager
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 2] verifying convocate07 is NOT in node list...");
    await login(page);
    await openNodeManager(page);

    let bodyText = await page.textContent("body");
    expect(bodyText).not.toContain(NODE_NAME);
    console.log("[step 2] confirmed: convocate07 not in node list");

    // ──────────────────────────────────────────────────────────────────────
    // Step 3: Use UI to provision the node
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 3] clicking Provision Node button...");
    await page.locator("button").filter({ hasText: /Provision Node/i }).click();
    await page.waitForTimeout(1000);

    // Fill in the provision form
    // Find the modal inputs by their labels
    const hostInput = page.locator("input").filter({ has: page.locator("..").locator("text=Host") }).first();
    const userInput = page.locator("input").filter({ has: page.locator("..").locator("text=SSH User") }).first();
    const passInput = page.locator("input[type='password']").last();
    const locationInput = page.locator("input").filter({ has: page.locator("..").locator("text=Location") }).first();

    // Fall back to filling by order if label-based selection doesn't work
    const allModalInputs = page.locator("[role='dialog'] input, [class*='modal'] input");
    const modalInputCount = await allModalInputs.count();

    if (modalInputCount >= 4) {
      await allModalInputs.nth(0).fill(NODE_IP);
      await allModalInputs.nth(1).fill(NODE_USER);
      await allModalInputs.nth(2).fill(NODE_PASS);
      await allModalInputs.nth(3).fill(NODE_LOCATION);
    } else {
      // Try with all visible inputs in the page
      const pageInputs = page.locator("input:visible");
      const count = await pageInputs.count();
      // The provision dialog inputs come after the login inputs
      // Find inputs that are empty (not the login form)
      for (let i = 0; i < count; i++) {
        const val = await pageInputs.nth(i).inputValue();
        if (val === "") {
          const placeholder = await pageInputs.nth(i).getAttribute("placeholder");
          if (placeholder?.includes("192.168")) {
            await pageInputs.nth(i).fill(NODE_IP);
          } else if (placeholder?.includes("root")) {
            await pageInputs.nth(i).fill(NODE_USER);
          } else if (placeholder?.includes("password")) {
            await pageInputs.nth(i).fill(NODE_PASS);
          } else if (placeholder?.includes("rack") || placeholder?.includes("east")) {
            await pageInputs.nth(i).fill(NODE_LOCATION);
          }
        }
      }
    }

    // Click the Provision button in the dialog
    await page.locator("button").filter({ hasText: /^Provision$/i }).click();
    console.log("[step 3] provision request submitted");

    // ──────────────────────────────────────────────────────────────────────
    // Step 4: Wait for convocate07 to appear in Node Manager as Ready
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 4] waiting for convocate07 to come online...");

    // Poll the API directly for up to 5 minutes
    let nodeFound = false;
    const deadline = Date.now() + 300_000;
    while (Date.now() < deadline) {
      const res = await request.get(`${APP}/api/v1/nmgr/node?limit=100`, {
        headers: { Authorization: "Bearer mock-token" },
      });
      const body = await res.json();
      const node = (body.items || []).find(
        (n: { id: string }) => n.id === NODE_NAME
      );
      if (node && node.status === "Ready") {
        nodeFound = true;
        // Verify memory and disk
        expect(node.memTotalGB).toBeGreaterThan(1); // 2 GB VM
        expect(node.diskTotalGB).toBeGreaterThan(5); // 20 GB disk
        console.log(
          `[step 4] ${NODE_NAME} is Ready — mem: ${node.memTotalGB.toFixed(1)} GB, disk: ${node.diskTotalGB.toFixed(1)} GB`
        );
        break;
      }
      await page.waitForTimeout(10_000);
    }
    expect(nodeFound).toBe(true);

    // Refresh the UI and verify the node appears
    await page.reload();
    await login(page);
    await openNodeManager(page);
    bodyText = await page.textContent("body");
    expect(bodyText).toContain(NODE_NAME);

    // ──────────────────────────────────────────────────────────────────────
    // Step 5: Verify via kubectl that convocate07 is properly configured
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 5] verifying node via kubectl...");

    const kubectlNode = ssh(`kubectl get node ${NODE_NAME} -o wide`);
    expect(kubectlNode).toContain(NODE_NAME);
    expect(kubectlNode).toContain("Ready");
    expect(kubectlNode).toContain(NODE_IP);

    // Check node labels
    const labels = ssh(
      `kubectl get node ${NODE_NAME} -o jsonpath='{.metadata.labels}'`
    );
    expect(labels).toContain("kubernetes.io/os");

    // Verify kubelet is running on the node
    const kubeletCheck = ssh(
      `kubectl get node ${NODE_NAME} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'`
    );
    expect(kubeletCheck).toBe("True");
    console.log("[step 5] kubectl verification passed");

    // ──────────────────────────────────────────────────────────────────────
    // Step 6: Verify the Cordon button works
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 6] testing Cordon button...");

    // Find the convocate07 row and click its Cordon button
    const cordonBtn = page.locator("tr", { hasText: NODE_NAME }).locator("button", { hasText: /Cordon/i });
    await cordonBtn.click();
    await page.waitForTimeout(500);

    // Confirm the cordon action in the dialog
    await page.locator("button").filter({ hasText: /Confirm/i }).click();
    await page.waitForTimeout(3000);

    // Verify via kubectl
    const cordonStatus = ssh(
      `kubectl get node ${NODE_NAME} -o jsonpath='{.spec.unschedulable}'`
    );
    expect(cordonStatus).toBe("true");

    // Verify in the API response
    const cordonRes = await request.get(`${APP}/api/v1/nmgr/node?limit=100`, {
      headers: { Authorization: "Bearer mock-token" },
    });
    const cordonBody = await cordonRes.json();
    const cordonedNode = (cordonBody.items || []).find(
      (n: { id: string }) => n.id === NODE_NAME
    );
    expect(cordonedNode.status).toBe("SchedulingDisabled");
    console.log("[step 6] Cordon verified");

    // ──────────────────────────────────────────────────────────────────────
    // Step 7: Verify the Delete button removes convocate07 from K8s
    //         but the VM remains running
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 7] testing Delete button...");

    // The UI should still show Node Manager from step 6.
    // Wait for the node list to refresh after cordon.
    await page.waitForTimeout(3000);

    // Find and click Delete for convocate07
    const deleteBtn = page.locator("tr", { hasText: NODE_NAME }).locator("button", { hasText: /Delete/i });
    await deleteBtn.click();
    await page.waitForTimeout(500);

    // Confirm the delete action
    await page.locator("button").filter({ hasText: /Confirm/i }).click();
    await page.waitForTimeout(10_000); // Wait for drain + delete

    // Verify node is gone from K8s
    const nodesAfterDelete = ssh("kubectl get nodes -o name");
    expect(nodesAfterDelete).not.toContain(NODE_NAME);

    // Verify the VM is still running (SSH should still work)
    const vmStatusAfter = ssh(
      `cd /home/${SVR00_USER}/git/svr00 && vagrant status ${NODE_NAME} | grep ${NODE_NAME}`
    );
    expect(vmStatusAfter).toContain("running");
    console.log("[step 7] Delete verified — node removed from K8s, VM still running");

    // ──────────────────────────────────────────────────────────────────────
    // Step 8: Cleanup is handled by afterAll
    // ──────────────────────────────────────────────────────────────────────
    console.log("[step 8] test complete — cleanup will run in afterAll");
  });
});
