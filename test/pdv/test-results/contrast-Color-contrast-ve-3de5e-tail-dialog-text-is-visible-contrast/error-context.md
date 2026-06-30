# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: contrast.spec.ts >> Color contrast verification >> Node Manager detail dialog text is visible
- Location: tests/contrast.spec.ts:186:7

# Error details

```
Error: Contrast failures in Node Manager detail dialog:
  "Start" — fg:rgb(255, 255, 255) bg:rgb(182, 208, 250) ratio:1.57 (interactive)

expect(received).toBe(expected) // Object.is equality

Expected: 0
Received: 1
```

# Page snapshot

```yaml
- generic [ref=e3]:
  - menubar "System panel" [ref=e5]:
    - generic [ref=e6]:
      - button "Activities" [ref=e7] [cursor=pointer]
      - generic [ref=e8]: Node Manager
    - timer "System clock" [ref=e10]:
      - generic [ref=e11]: Tue, Jun 30
      - generic [ref=e12]: 15:37:45
    - button "Mock Admin" [ref=e15] [cursor=pointer]:
      - generic [ref=e16]: M
      - generic [ref=e17]: Mock Admin
      - generic [ref=e18]: ▾
  - generic [ref=e19]:
    - toolbar "Application launcher" [ref=e21]:
      - button "Node Manager" [pressed] [ref=e22] [cursor=pointer]:
        - img "Node Manager" [ref=e23]
      - button "Agent Manager" [ref=e24] [cursor=pointer]:
        - img "Agent Manager" [ref=e25]
      - button "Convocate Project Board" [ref=e26] [cursor=pointer]:
        - img "Convocate Project Board" [ref=e27]
      - button "Code IDE" [ref=e28] [cursor=pointer]:
        - img "Code IDE" [ref=e29]
      - button "Access Control" [ref=e30] [cursor=pointer]:
        - img "Access Control" [ref=e31]
      - button "Repo Manager" [ref=e32] [cursor=pointer]:
        - img "Repo Manager" [ref=e33]
      - button "Support Tool" [ref=e34] [cursor=pointer]:
        - img "Support Tool" [ref=e35]
    - main [ref=e36]:
      - application "Desktop workspace" [ref=e37]:
        - dialog "Node Manager" [ref=e40]:
          - toolbar "Node Manager window controls" [ref=e49]:
            - generic [ref=e50]: Node Manager
            - generic [ref=e51]:
              - button "Minimize" [ref=e52] [cursor=pointer]
              - button "Maximize" [ref=e53] [cursor=pointer]
              - button "Close" [ref=e54] [cursor=pointer]
          - generic [ref=e57]:
            - generic [ref=e58]:
              - generic [ref=e59]:
                - generic [ref=e60]: Nodes
                - generic "Live updates active" [ref=e61]
              - generic [ref=e62]:
                - button "Provision Node" [ref=e63] [cursor=pointer]
                - button "Refresh" [ref=e64] [cursor=pointer]
            - grid [ref=e68]:
              - rowgroup [ref=e69]:
                - row "Name Location IP Status Agents Load (1/5/15m) Memory Disk" [ref=e70]:
                  - columnheader "Name" [ref=e71]
                  - columnheader "Location" [ref=e72]
                  - columnheader "IP" [ref=e73]
                  - columnheader "Status" [ref=e74]
                  - columnheader "Agents" [ref=e75]
                  - columnheader "Load (1/5/15m)" [ref=e76]
                  - columnheader "Memory" [ref=e77]
                  - columnheader "Disk" [ref=e78]
                  - columnheader [ref=e79]
              - rowgroup [ref=e80]:
                - row "convocate01 unspecified 192.168.56.11 Ready 0 0.59 / 0.66 / 0.57 1.6 / 15.6 GB 6.2 / 57.1 GB Stop" [ref=e81]:
                  - gridcell "convocate01" [ref=e82]:
                    - link "convocate01" [active] [ref=e83] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e84]
                  - gridcell "192.168.56.11" [ref=e85]
                  - gridcell "Ready" [ref=e86]:
                    - generic [ref=e88]: Ready
                  - gridcell "0" [ref=e89]
                  - gridcell "0.59 / 0.66 / 0.57" [ref=e90]
                  - gridcell "1.6 / 15.6 GB" [ref=e91]
                  - gridcell "6.2 / 57.1 GB" [ref=e92]
                  - gridcell "Stop" [ref=e93]:
                    - button "Stop" [ref=e94] [cursor=pointer]
                - row "convocate02 unspecified 192.168.56.12 Ready 0 0.43 / 0.46 / 0.42 1.3 / 15.6 GB 5.8 / 57.1 GB Stop" [ref=e95]:
                  - gridcell "convocate02" [ref=e96]:
                    - link "convocate02" [ref=e97] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e98]
                  - gridcell "192.168.56.12" [ref=e99]
                  - gridcell "Ready" [ref=e100]:
                    - generic [ref=e102]: Ready
                  - gridcell "0" [ref=e103]
                  - gridcell "0.43 / 0.46 / 0.42" [ref=e104]
                  - gridcell "1.3 / 15.6 GB" [ref=e105]
                  - gridcell "5.8 / 57.1 GB" [ref=e106]
                  - gridcell "Stop" [ref=e107]:
                    - button "Stop" [ref=e108] [cursor=pointer]
                - row "convocate03 unspecified 192.168.56.13 Ready 0 0.30 / 0.41 / 0.43 1.2 / 15.6 GB 5.8 / 57.1 GB Stop" [ref=e109]:
                  - gridcell "convocate03" [ref=e110]:
                    - link "convocate03" [ref=e111] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e112]
                  - gridcell "192.168.56.13" [ref=e113]
                  - gridcell "Ready" [ref=e114]:
                    - generic [ref=e116]: Ready
                  - gridcell "0" [ref=e117]
                  - gridcell "0.30 / 0.41 / 0.43" [ref=e118]
                  - gridcell "1.2 / 15.6 GB" [ref=e119]
                  - gridcell "5.8 / 57.1 GB" [ref=e120]
                  - gridcell "Stop" [ref=e121]:
                    - button "Stop" [ref=e122] [cursor=pointer]
                - row "convocate04 unspecified 192.168.56.14 Ready 0 0.43 / 0.30 / 0.17 0.9 / 15.6 GB 5.6 / 57.1 GB Stop" [ref=e123]:
                  - gridcell "convocate04" [ref=e124]:
                    - link "convocate04" [ref=e125] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e126]
                  - gridcell "192.168.56.14" [ref=e127]
                  - gridcell "Ready" [ref=e128]:
                    - generic [ref=e130]: Ready
                  - gridcell "0" [ref=e131]
                  - gridcell "0.43 / 0.30 / 0.17" [ref=e132]
                  - gridcell "0.9 / 15.6 GB" [ref=e133]
                  - gridcell "5.6 / 57.1 GB" [ref=e134]
                  - gridcell "Stop" [ref=e135]:
                    - button "Stop" [ref=e136] [cursor=pointer]
                - row "convocate05 unspecified 192.168.56.15 Ready 0 0.87 / 0.35 / 0.17 1.0 / 15.6 GB 10.4 / 57.1 GB Stop" [ref=e137]:
                  - gridcell "convocate05" [ref=e138]:
                    - link "convocate05" [ref=e139] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e140]
                  - gridcell "192.168.56.15" [ref=e141]
                  - gridcell "Ready" [ref=e142]:
                    - generic [ref=e144]: Ready
                  - gridcell "0" [ref=e145]
                  - gridcell "0.87 / 0.35 / 0.17" [ref=e146]
                  - gridcell "1.0 / 15.6 GB" [ref=e147]
                  - gridcell "10.4 / 57.1 GB" [ref=e148]
                  - gridcell "Stop" [ref=e149]:
                    - button "Stop" [ref=e150] [cursor=pointer]
                - row "convocate06 unspecified 192.168.56.16 SchedulingDisabled 0 0.15 / 0.27 / 0.21 1.1 / 15.6 GB 9.4 / 57.1 GB Start" [ref=e151]:
                  - gridcell "convocate06" [ref=e152]:
                    - link "convocate06" [ref=e153] [cursor=pointer]:
                      - /url: "#"
                  - gridcell "unspecified" [ref=e154]
                  - gridcell "192.168.56.16" [ref=e155]
                  - gridcell "SchedulingDisabled" [ref=e156]:
                    - generic [ref=e158]: SchedulingDisabled
                  - gridcell "0" [ref=e159]
                  - gridcell "0.15 / 0.27 / 0.21" [ref=e160]
                  - gridcell "1.1 / 15.6 GB" [ref=e161]
                  - gridcell "9.4 / 57.1 GB" [ref=e162]
                  - gridcell "Start" [ref=e163]:
                    - button "Start" [ref=e164] [cursor=pointer]
            - generic [ref=e166]: 6 nodes
            - 'dialog "Node: convocate01" [ref=e167]':
              - document [ref=e168]:
                - generic [ref=e169]:
                  - 'heading "Node: convocate01" [level=2] [ref=e170]'
                  - button "Close modal" [ref=e171] [cursor=pointer]: ×
                - generic [ref=e173]:
                  - tablist "Tabs" [ref=e174]:
                    - tab "Overview" [selected] [ref=e175] [cursor=pointer]
                    - tab "Notes" [ref=e176] [cursor=pointer]
                    - tab "Agents" [ref=e177] [cursor=pointer]
                  - tabpanel "Overview" [ref=e178]:
                    - generic [ref=e179]:
                      - generic [ref=e180]:
                        - generic [ref=e181]:
                          - generic [ref=e182]: STATUS
                          - generic [ref=e184]: Ready
                        - generic [ref=e185]:
                          - generic [ref=e186]: IP ADDRESS
                          - generic [ref=e187]: 192.168.56.11
                        - generic [ref=e188]:
                          - generic [ref=e189]: LOCATION
                          - generic [ref=e190]: unspecified
                        - generic [ref=e191]:
                          - generic [ref=e192]: AGENTS
                          - generic [ref=e193]: "0"
                        - generic [ref=e194]:
                          - generic [ref=e195]: LOAD AVG
                          - generic [ref=e196]: 0.59 / 0.66 / 0.57
                        - generic [ref=e197]:
                          - generic [ref=e198]: MEMORY
                          - generic [ref=e199]: 1.6 / 15.6 GB
                        - generic [ref=e200]:
                          - generic [ref=e201]: DISK
                          - generic [ref=e202]: 6.2 / 57.1 GB
                      - generic [ref=e204]: TAGS
                      - generic [ref=e213]:
                        - button "Stop" [ref=e214] [cursor=pointer]
                        - button "Delete" [ref=e215] [cursor=pointer]
```

# Test source

```ts
  58  |         return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  59  |       });
  60  |       return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs;
  61  |     }
  62  | 
  63  |     function contrastRatio(l1: number, l2: number): number {
  64  |       const lighter = Math.max(l1, l2);
  65  |       const darker = Math.min(l1, l2);
  66  |       return (lighter + 0.05) / (darker + 0.05);
  67  |     }
  68  | 
  69  |     function effectiveBg(el: Element): string {
  70  |       let current: Element | null = el;
  71  |       while (current) {
  72  |         const style = window.getComputedStyle(current);
  73  |         const bg = style.backgroundColor;
  74  |         const parsed = parseRgb(bg);
  75  |         if (parsed) {
  76  |           const alphaMatch = bg.match(/rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*([\d.]+)/);
  77  |           const alpha = alphaMatch ? parseFloat(alphaMatch[1]) : 1;
  78  |           if (alpha > 0.1) return bg;
  79  |         }
  80  |         current = current.parentElement;
  81  |       }
  82  |       return "rgb(0, 0, 0)";
  83  |     }
  84  | 
  85  |     const container = document.querySelector(selector);
  86  |     if (!container) return [];
  87  | 
  88  |     const results: { text: string; fg: string; bg: string; ratio: number; isInteractive: boolean }[] = [];
  89  |     const seen = new Set<string>();
  90  | 
  91  |     const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
  92  |     let node: Node | null;
  93  |     while ((node = walker.nextNode())) {
  94  |       const text = (node.textContent || "").trim();
  95  |       if (!text || text.length > 100) continue;
  96  | 
  97  |       const el = node.parentElement;
  98  |       if (!el) continue;
  99  | 
  100 |       const style = window.getComputedStyle(el);
  101 |       if (style.display === "none" || style.visibility === "hidden" || style.opacity === "0")
  102 |         continue;
  103 | 
  104 |       const fg = style.color;
  105 |       const bg = effectiveBg(el);
  106 | 
  107 |       const key = `${fg}|${bg}`;
  108 |       if (seen.has(key)) continue;
  109 |       seen.add(key);
  110 | 
  111 |       const fgRgb = parseRgb(fg);
  112 |       const bgRgb = parseRgb(bg);
  113 |       if (!fgRgb || !bgRgb) continue;
  114 | 
  115 |       const fgLum = relativeLuminance(...fgRgb);
  116 |       const bgLum = relativeLuminance(...bgRgb);
  117 |       const ratio = contrastRatio(fgLum, bgLum);
  118 | 
  119 |       // Determine if the element is interactive (button, link, tab)
  120 |       const tagName = el.tagName.toLowerCase();
  121 |       const role = el.getAttribute("role");
  122 |       const isInteractive =
  123 |         tagName === "button" || tagName === "a" ||
  124 |         role === "button" || role === "tab" || role === "link" ||
  125 |         !!el.closest("button, a, [role=button], [role=tab]");
  126 | 
  127 |       results.push({
  128 |         text: text.substring(0, 40),
  129 |         fg,
  130 |         bg,
  131 |         ratio: Math.round(ratio * 100) / 100,
  132 |         isInteractive,
  133 |       });
  134 |     }
  135 | 
  136 |     return results;
  137 |   }, containerSelector);
  138 | }
  139 | 
  140 | /**
  141 |  * Assert no contrast violations.
  142 |  * Interactive elements (buttons, links, tabs) use 3:1 threshold.
  143 |  * Normal text uses 4.5:1 threshold.
  144 |  */
  145 | function assertContrast(
  146 |   results: { text: string; fg: string; bg: string; ratio: number; isInteractive: boolean }[],
  147 |   context: string
  148 | ) {
  149 |   const failures = results.filter((r) => {
  150 |     const threshold = r.isInteractive ? MIN_CONTRAST_INTERACTIVE : MIN_CONTRAST;
  151 |     return r.ratio < threshold;
  152 |   });
  153 | 
  154 |   if (failures.length > 0) {
  155 |     const report = failures
  156 |       .map((f) => `  "${f.text}" — fg:${f.fg} bg:${f.bg} ratio:${f.ratio} ${f.isInteractive ? "(interactive)" : "(text)"}`)
  157 |       .join("\n");
> 158 |     expect(failures.length, `Contrast failures in ${context}:\n${report}`).toBe(0);
      |                                                                            ^ Error: Contrast failures in Node Manager detail dialog:
  159 |   }
  160 | }
  161 | 
  162 | // ---------------------------------------------------------------------------
  163 | // Tests
  164 | // ---------------------------------------------------------------------------
  165 | 
  166 | test.describe("Color contrast verification", () => {
  167 |   test("login screen text is visible against its background", async ({ page }) => {
  168 |     await page.goto("/");
  169 |     await expect(page.locator("text=Convocate")).toBeVisible({ timeout: 10000 });
  170 | 
  171 |     const results = await auditContrast(page, "#app");
  172 |     assertContrast(results, "login screen");
  173 |   });
  174 | 
  175 |   test("Node Manager grid text is visible against dark background", async ({ page }) => {
  176 |     await login(page);
  177 | 
  178 |     await page.locator('[data-dock-item-id="nmgr"]').click();
  179 |     await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
  180 |     await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  181 | 
  182 |     const results = await auditContrast(page, '[data-testid="node-manager"]');
  183 |     assertContrast(results, "Node Manager grid");
  184 |   });
  185 | 
  186 |   test("Node Manager detail dialog text is visible", async ({ page }) => {
  187 |     await login(page);
  188 | 
  189 |     await page.locator('[data-dock-item-id="nmgr"]').click();
  190 |     await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
  191 |     await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  192 | 
  193 |     await page.locator('[data-testid="node-manager"] a').first().click();
  194 |     await expect(page.locator('text=/Node: /')).toBeVisible({ timeout: 5000 });
  195 | 
  196 |     // Audit the Node Manager area (excludes SpecifyJS chrome we can't control)
  197 |     const results = await auditContrast(page, '[data-testid="node-manager"]');
  198 |     assertContrast(results, "Node Manager detail dialog");
  199 |   });
  200 | 
  201 |   test("no metrics show impossible negative values", async ({ page }) => {
  202 |     await login(page);
  203 | 
  204 |     await page.locator('[data-dock-item-id="nmgr"]').click();
  205 |     await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
  206 |     await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  207 | 
  208 |     // Check that no cell in the grid shows negative values for metrics
  209 |     const cellTexts = await page.locator('[data-testid="node-manager"] td').allTextContents();
  210 |     for (const text of cellTexts) {
  211 |       // Metrics columns should never show negative numbers
  212 |       if (text.includes("-1.0") || text.includes("-1.00")) {
  213 |         expect(text, `Found impossible metric value: "${text}"`).not.toContain("-1");
  214 |       }
  215 |     }
  216 |   });
  217 | 
  218 |   test("metrics show dash when data unavailable", async ({ page }) => {
  219 |     await login(page);
  220 | 
  221 |     await page.locator('[data-dock-item-id="nmgr"]').click();
  222 |     await expect(page.locator('[data-testid="node-manager"]')).toBeVisible({ timeout: 10000 });
  223 |     await expect(page.locator('text=/\\d+ nodes?/')).toBeVisible({ timeout: 10000 });
  224 | 
  225 |     // Verify that metrics are either valid positive numbers or "—"
  226 |     const cellTexts = await page.locator('[data-testid="node-manager"] td').allTextContents();
  227 |     const metricPattern = /^-?\d+\.\d+\s*\/\s*\d+\.\d+\s*GB$/;
  228 |     for (const text of cellTexts) {
  229 |       if (metricPattern.test(text.trim())) {
  230 |         // If it matches the "X.X / X.X GB" pattern, values must be non-negative
  231 |         const nums = text.match(/-?\d+\.\d+/g);
  232 |         if (nums) {
  233 |           for (const n of nums) {
  234 |             expect(parseFloat(n), `Negative metric value in "${text}"`).toBeGreaterThanOrEqual(0);
  235 |           }
  236 |         }
  237 |       }
  238 |     }
  239 |   });
  240 | });
  241 | 
```