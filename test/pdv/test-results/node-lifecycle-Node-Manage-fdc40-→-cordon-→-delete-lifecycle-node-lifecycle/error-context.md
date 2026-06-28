# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: node-lifecycle.spec.ts >> Node Manager — full provision/cordon/delete lifecycle >> provision → verify → cordon → delete lifecycle
- Location: tests/node-lifecycle.spec.ts:85:7

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: true
Received: false
```

# Test source

```ts
  99  |     expect(vmStatus).toContain("running");
  100 |     console.log("[step 1] VM is running");
  101 | 
  102 |     // Enable sudo for the convocate user via vagrant user (has sudo)
  103 |     console.log("[step 1b] enabling sudo for convocate user...");
  104 |     ssh(
  105 |       `cd /home/${SVR00_USER}/git/svr00 && vagrant ssh ${NODE_NAME} -c "sudo sh -c 'echo \\\"convocate ALL=(ALL) NOPASSWD:ALL\\\" > /etc/sudoers.d/convocate && chmod 0440 /etc/sudoers.d/convocate'"`,
  106 |       30_000
  107 |     );
  108 |     console.log("[step 1b] sudo configured");
  109 | 
  110 |     // ──────────────────────────────────────────────────────────────────────
  111 |     // Step 2: Verify convocate07 does NOT appear in Node Manager
  112 |     // ──────────────────────────────────────────────────────────────────────
  113 |     console.log("[step 2] verifying convocate07 is NOT in node list...");
  114 |     await login(page);
  115 |     await openNodeManager(page);
  116 | 
  117 |     let bodyText = await page.textContent("body");
  118 |     expect(bodyText).not.toContain(NODE_NAME);
  119 |     console.log("[step 2] confirmed: convocate07 not in node list");
  120 | 
  121 |     // ──────────────────────────────────────────────────────────────────────
  122 |     // Step 3: Use UI to provision the node
  123 |     // ──────────────────────────────────────────────────────────────────────
  124 |     console.log("[step 3] clicking Provision Node button...");
  125 |     await page.locator("button").filter({ hasText: /Provision Node/i }).click();
  126 |     await page.waitForTimeout(1000);
  127 | 
  128 |     // Fill in the provision form
  129 |     // Find the modal inputs by their labels
  130 |     const hostInput = page.locator("input").filter({ has: page.locator("..").locator("text=Host") }).first();
  131 |     const userInput = page.locator("input").filter({ has: page.locator("..").locator("text=SSH User") }).first();
  132 |     const passInput = page.locator("input[type='password']").last();
  133 |     const locationInput = page.locator("input").filter({ has: page.locator("..").locator("text=Location") }).first();
  134 | 
  135 |     // Fall back to filling by order if label-based selection doesn't work
  136 |     const allModalInputs = page.locator("[role='dialog'] input, [class*='modal'] input");
  137 |     const modalInputCount = await allModalInputs.count();
  138 | 
  139 |     if (modalInputCount >= 4) {
  140 |       await allModalInputs.nth(0).fill(NODE_IP);
  141 |       await allModalInputs.nth(1).fill(NODE_USER);
  142 |       await allModalInputs.nth(2).fill(NODE_PASS);
  143 |       await allModalInputs.nth(3).fill(NODE_LOCATION);
  144 |     } else {
  145 |       // Try with all visible inputs in the page
  146 |       const pageInputs = page.locator("input:visible");
  147 |       const count = await pageInputs.count();
  148 |       // The provision dialog inputs come after the login inputs
  149 |       // Find inputs that are empty (not the login form)
  150 |       for (let i = 0; i < count; i++) {
  151 |         const val = await pageInputs.nth(i).inputValue();
  152 |         if (val === "") {
  153 |           const placeholder = await pageInputs.nth(i).getAttribute("placeholder");
  154 |           if (placeholder?.includes("192.168")) {
  155 |             await pageInputs.nth(i).fill(NODE_IP);
  156 |           } else if (placeholder?.includes("root")) {
  157 |             await pageInputs.nth(i).fill(NODE_USER);
  158 |           } else if (placeholder?.includes("password")) {
  159 |             await pageInputs.nth(i).fill(NODE_PASS);
  160 |           } else if (placeholder?.includes("rack") || placeholder?.includes("east")) {
  161 |             await pageInputs.nth(i).fill(NODE_LOCATION);
  162 |           }
  163 |         }
  164 |       }
  165 |     }
  166 | 
  167 |     // Click the Provision button in the dialog
  168 |     await page.locator("button").filter({ hasText: /^Provision$/i }).click();
  169 |     console.log("[step 3] provision request submitted");
  170 | 
  171 |     // ──────────────────────────────────────────────────────────────────────
  172 |     // Step 4: Wait for convocate07 to appear in Node Manager as Ready
  173 |     // ──────────────────────────────────────────────────────────────────────
  174 |     console.log("[step 4] waiting for convocate07 to come online...");
  175 | 
  176 |     // Poll the API directly for up to 5 minutes
  177 |     let nodeFound = false;
  178 |     const deadline = Date.now() + 300_000;
  179 |     while (Date.now() < deadline) {
  180 |       const res = await request.get(`${APP}/api/v1/nmgr/node?limit=100`, {
  181 |         headers: { Authorization: "Bearer mock-token" },
  182 |       });
  183 |       const body = await res.json();
  184 |       const node = (body.items || []).find(
  185 |         (n: { id: string }) => n.id === NODE_NAME
  186 |       );
  187 |       if (node && node.status === "Ready") {
  188 |         nodeFound = true;
  189 |         // Verify memory and disk
  190 |         expect(node.memTotalGB).toBeGreaterThan(1); // 2 GB VM
  191 |         expect(node.diskTotalGB).toBeGreaterThan(5); // 20 GB disk
  192 |         console.log(
  193 |           `[step 4] ${NODE_NAME} is Ready — mem: ${node.memTotalGB.toFixed(1)} GB, disk: ${node.diskTotalGB.toFixed(1)} GB`
  194 |         );
  195 |         break;
  196 |       }
  197 |       await page.waitForTimeout(10_000);
  198 |     }
> 199 |     expect(nodeFound).toBe(true);
      |                       ^ Error: expect(received).toBe(expected) // Object.is equality
  200 | 
  201 |     // Refresh the UI and verify the node appears
  202 |     await page.reload();
  203 |     await login(page);
  204 |     await openNodeManager(page);
  205 |     bodyText = await page.textContent("body");
  206 |     expect(bodyText).toContain(NODE_NAME);
  207 | 
  208 |     // ──────────────────────────────────────────────────────────────────────
  209 |     // Step 5: Verify via kubectl that convocate07 is properly configured
  210 |     // ──────────────────────────────────────────────────────────────────────
  211 |     console.log("[step 5] verifying node via kubectl...");
  212 | 
  213 |     const kubectlNode = ssh(`kubectl get node ${NODE_NAME} -o wide`);
  214 |     expect(kubectlNode).toContain(NODE_NAME);
  215 |     expect(kubectlNode).toContain("Ready");
  216 |     expect(kubectlNode).toContain(NODE_IP);
  217 | 
  218 |     // Check node labels
  219 |     const labels = ssh(
  220 |       `kubectl get node ${NODE_NAME} -o jsonpath='{.metadata.labels}'`
  221 |     );
  222 |     expect(labels).toContain("kubernetes.io/os");
  223 | 
  224 |     // Verify kubelet is running on the node
  225 |     const kubeletCheck = ssh(
  226 |       `kubectl get node ${NODE_NAME} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'`
  227 |     );
  228 |     expect(kubeletCheck).toBe("True");
  229 |     console.log("[step 5] kubectl verification passed");
  230 | 
  231 |     // ──────────────────────────────────────────────────────────────────────
  232 |     // Step 6: Verify the Cordon button works
  233 |     // ──────────────────────────────────────────────────────────────────────
  234 |     console.log("[step 6] testing Cordon button...");
  235 | 
  236 |     // Find the convocate07 row and click its Cordon button
  237 |     const cordonBtn = page.locator("tr", { hasText: NODE_NAME }).locator("button", { hasText: /Cordon/i });
  238 |     await cordonBtn.click();
  239 |     await page.waitForTimeout(500);
  240 | 
  241 |     // Confirm the cordon action in the dialog
  242 |     await page.locator("button").filter({ hasText: /Confirm/i }).click();
  243 |     await page.waitForTimeout(3000);
  244 | 
  245 |     // Verify via kubectl
  246 |     const cordonStatus = ssh(
  247 |       `kubectl get node ${NODE_NAME} -o jsonpath='{.spec.unschedulable}'`
  248 |     );
  249 |     expect(cordonStatus).toBe("true");
  250 | 
  251 |     // Verify in the API response
  252 |     const cordonRes = await request.get(`${APP}/api/v1/nmgr/node?limit=100`, {
  253 |       headers: { Authorization: "Bearer mock-token" },
  254 |     });
  255 |     const cordonBody = await cordonRes.json();
  256 |     const cordonedNode = (cordonBody.items || []).find(
  257 |       (n: { id: string }) => n.id === NODE_NAME
  258 |     );
  259 |     expect(cordonedNode.status).toBe("SchedulingDisabled");
  260 |     console.log("[step 6] Cordon verified");
  261 | 
  262 |     // ──────────────────────────────────────────────────────────────────────
  263 |     // Step 7: Verify the Delete button removes convocate07 from K8s
  264 |     //         but the VM remains running
  265 |     // ──────────────────────────────────────────────────────────────────────
  266 |     console.log("[step 7] testing Delete button...");
  267 | 
  268 |     // Reload to get fresh UI with Uncordon button visible
  269 |     await page.reload();
  270 |     await login(page);
  271 |     await openNodeManager(page);
  272 |     await page.waitForTimeout(2000);
  273 | 
  274 |     // Find and click Delete for convocate07
  275 |     const deleteBtn = page.locator("tr", { hasText: NODE_NAME }).locator("button", { hasText: /Delete/i });
  276 |     await deleteBtn.click();
  277 |     await page.waitForTimeout(500);
  278 | 
  279 |     // Confirm the delete action
  280 |     await page.locator("button").filter({ hasText: /Confirm/i }).click();
  281 |     await page.waitForTimeout(10_000); // Wait for drain + delete
  282 | 
  283 |     // Verify node is gone from K8s
  284 |     const nodesAfterDelete = ssh("kubectl get nodes -o name");
  285 |     expect(nodesAfterDelete).not.toContain(NODE_NAME);
  286 | 
  287 |     // Verify the VM is still running (SSH should still work)
  288 |     const vmStatusAfter = ssh(
  289 |       `cd /home/${SVR00_USER}/git/svr00 && vagrant status ${NODE_NAME} | grep ${NODE_NAME}`
  290 |     );
  291 |     expect(vmStatusAfter).toContain("running");
  292 |     console.log("[step 7] Delete verified — node removed from K8s, VM still running");
  293 | 
  294 |     // ──────────────────────────────────────────────────────────────────────
  295 |     // Step 8: Cleanup is handled by afterAll
  296 |     // ──────────────────────────────────────────────────────────────────────
  297 |     console.log("[step 8] test complete — cleanup will run in afterAll");
  298 |   });
  299 | });
```