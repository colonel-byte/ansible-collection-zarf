/**
 * Copyright 2026 colonel-byte
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

document.addEventListener("DOMContentLoaded", () => {
  const isDark = document.documentElement.classList.contains("navy") ||
    document.documentElement.classList.contains("coal") ||
    document.documentElement.classList.contains("ayu");

  document.querySelectorAll("pre code.language-mermaid").forEach((el) => {
    const pre = el.parentElement;
    const div = document.createElement("div");
    div.className = "mermaid";
    div.style.display = "flex";
    div.style.justifyContent = "center";
    div.style.padding = "1rem";
    div.style.margin = "1rem 0";
    div.style.borderRadius = "8px";
    div.style.backgroundColor = "var(--table-alternate-bg, rgba(255, 255, 255, 0.05))";
    div.style.border = "1px solid var(--table-border-color, rgba(255, 255, 255, 0.1))";
    div.textContent = el.textContent;
    pre.replaceWith(div);
  });

  const raiseLabelsAboveArrows = () => {
    document.querySelectorAll(".mermaid svg").forEach((svg) => {
      const edgeLabels = svg.querySelector("g.edgeLabels");
      if (edgeLabels) {
        svg.appendChild(edgeLabels);
      }
      svg.querySelectorAll("g.edgeLabel").forEach((lbl) => {
        lbl.parentNode.appendChild(lbl);
      });
    });
  };

  if (typeof mermaid !== "undefined") {
    mermaid.initialize({
      startOnLoad: false,
      theme: isDark ? "dark" : "default",
      themeCSS: `
        .mermaid .edgeLabel, .mermaid .edgeLabel rect, .mermaid .edgeLabel span, .mermaid .edgeLabel p {
          position: relative;
        }
        .mermaid .edgeLabel rect {
          opacity: 0.95 !important;
          fill: var(--table-alternate-bg, rgba(30, 30, 30, 0.95)) !important;
        }
        .mermaid text, .mermaid span, .mermaid p {
          text-shadow: 0 0 1px #000, 0 0 2px #000, 0 0 3px #000;
          -webkit-text-stroke: 0.6px #000;
          paint-order: stroke fill;
        }
      `
    });

    mermaid.run().then(() => {
      raiseLabelsAboveArrows();
    }).catch(() => {
      raiseLabelsAboveArrows();
    });
  }
});
