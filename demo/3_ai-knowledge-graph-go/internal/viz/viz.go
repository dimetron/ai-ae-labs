// Package viz generates interactive HTML visualizations of knowledge graphs.
package viz

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dimetron/ai-knowledge-graph-go/internal/domain"
	"github.com/dimetron/ai-knowledge-graph-go/internal/graph"
)

// Render generates an interactive HTML visualization and writes it to outputFile.
func Render(kg *graph.KnowledgeGraph, outputFile string, edgeSmooth interface{}) error {
	if len(kg.Nodes) == 0 {
		return fmt.Errorf("no nodes to visualize")
	}

	slog.Info("rendering visualization",
		"nodes", len(kg.Nodes),
		"edges", len(kg.Edges),
		"output", outputFile,
	)

	nodesJSON, err := json.Marshal(kg.Nodes)
	if err != nil {
		return fmt.Errorf("marshal nodes: %w", err)
	}

	edgesJSON, err := json.Marshal(kg.Edges)
	if err != nil {
		return fmt.Errorf("marshal edges: %w", err)
	}

	edgeSmoothJSON := resolveEdgeSmooth(edgeSmooth)

	html := generateHTML(string(nodesJSON), string(edgesJSON), edgeSmoothJSON, kg.Stats, kg.Communities)

	if err := os.WriteFile(outputFile, []byte(html), 0o644); err != nil {
		return fmt.Errorf("write HTML %s: %w", outputFile, err)
	}

	slog.Info("visualization saved", "file", outputFile)
	return nil
}

func resolveEdgeSmooth(v interface{}) string {
	switch val := v.(type) {
	case bool:
		if val {
			return `{"type": "continuous"}`
		}
		return "false"
	case string:
		if strings.ToLower(val) == "false" {
			return "false"
		}
		return fmt.Sprintf(`{"type": "%s"}`, val)
	default:
		return "false"
	}
}

func generateHTML(nodesJSON, edgesJSON, edgeSmoothJSON string, stats domain.GraphStats, communities int) string {
	// Self-contained HTML with vis-network from CDN.
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Knowledge Graph - %d Nodes, %d Relationships, %d Communities</title>
<script type="text/javascript" src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
<style>
html, body { height: 100%%; margin: 0; padding: 0; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body.light-mode { background-color: white; color: black; }
body.dark-mode { background-color: #121212; color: #e0e0e0; }
.card { width: 100%%; height: 100vh; display: flex; flex-direction: column; }
body.dark-mode .card { background-color: #1e1e1e; }
body.dark-mode #mynetwork { background-color: #000000; }

#graph-controls { margin: 10px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
#graph-controls button { padding: 6px 12px; border: 1px solid #ccc; border-radius: 4px; cursor: pointer; background: #fff; font-size: 13px; }
#graph-controls button:hover { background: #f0f0f0; }
#graph-controls button.active { background: #0d6efd; color: white; border-color: #0d6efd; }
body.dark-mode #graph-controls button { background: #333; color: #e0e0e0; border-color: #555; }
body.dark-mode #graph-controls button:hover { background: #444; }
body.dark-mode #graph-controls button.active { background: #0d6efd; color: white; }

#stats-container { display: none; margin: 0 10px; padding: 10px 15px; border: 1px solid #ddd; border-radius: 5px; background: #f8f9fa; }
body.dark-mode #stats-container { background: #1e1e1e; border-color: #444; color: #e0e0e0; }

#filter-container { display: none; margin: 0 10px; padding: 10px; border: 1px solid #ddd; border-radius: 5px; background: #f8f9fa; }
body.dark-mode #filter-container { background: #1e1e1e; border-color: #444; }
#filter-container select { padding: 4px 8px; border: 1px solid #ccc; border-radius: 4px; font-size: 13px; }
body.dark-mode #filter-container select { background: #333; color: #e0e0e0; border-color: #555; }

.legend { font-size: 13px; display: flex; align-items: center; gap: 15px; }
.legend-line { display: inline-block; width: 30px; height: 2px; background: #666; margin-right: 4px; }
.legend-dash { display: inline-block; width: 30px; height: 0; border-top: 2px dashed #666; margin-right: 4px; }

#footer { position: absolute; bottom: 10px; left: 50%%; transform: translateX(-50%%); padding: 5px 10px; font-size: 0.85rem; background: rgba(255,255,255,0.7); border-radius: 4px; backdrop-filter: blur(2px); z-index: 100; }
body.dark-mode #footer { background: rgba(30,30,30,0.7); color: #adb5bd; }
#footer a { text-decoration: none; color: inherit; }
</style>
</head>
<body class="light-mode">

<div class="card">
  <div id="graph-controls">
    <div style="display: flex; gap: 6px; flex-wrap: wrap;">
      <button id="physics-toggle" class="active" onclick="togglePhysics()">Disable Physics</button>
      <button onclick="stabilizeNetwork()">Stabilize</button>
      <button id="theme-toggle" onclick="toggleDarkMode()">Dark Mode</button>
      <button id="labels-toggle" class="active" onclick="toggleLabels()">Hide Labels</button>
      <button id="filter-toggle" onclick="toggleFilter()">Show Filters</button>
      <button id="stats-toggle" onclick="toggleStats()">Stats</button>
    </div>
    <div class="legend">
      <span><strong>Edge Types:</strong></span>
      <span><span class="legend-line"></span> Extracted</span>
      <span><span class="legend-dash"></span> Inferred</span>
    </div>
  </div>

  <div id="stats-container">
    <h4 style="margin: 0 0 8px 0;">Graph Statistics</h4>
    <div style="display: flex; flex-wrap: wrap; gap: 20px;">
      <div><strong>Nodes:</strong> %d</div>
      <div><strong>Edges:</strong> %d</div>
      <div><strong>Extracted:</strong> %d</div>
      <div><strong>Inferred:</strong> %d</div>
      <div><strong>Communities:</strong> %d</div>
    </div>
  </div>

  <div id="filter-container">
    <div style="display: flex; gap: 8px; align-items: center;">
      <select id="node-select" onchange="selectNode(this.value)">
        <option value="">Select a Node</option>
      </select>
      <button onclick="resetSelection()">Reset</button>
    </div>
  </div>

  <div id="mynetwork" style="flex: 1;"></div>

  <div id="footer">
    <a href="https://github.com/dimetron/ai-knowledge-graph-go" target="_blank">AI Knowledge Graph (Go ADK 2.0)</a>
  </div>
</div>

<script>
var rawNodes = %s;
var rawEdges = %s;

var visNodes = new vis.DataSet(rawNodes.map(function(n) {
  return { id: n.id, label: n.label, color: n.color, size: n.size,
           title: n.label + " - Connections: " + n.degree, shape: "dot",
           font: { color: "#000000", strokeWidth: 0 } };
}));

var visEdges = new vis.DataSet(rawEdges.map(function(e, i) {
  return { id: i, from: e.from, to: e.to, label: e.label, title: e.label,
           arrows: "to", dashes: e.inferred || false,
           color: e.inferred ? "#555555" : undefined };
}));

var container = document.getElementById("mynetwork");
var network = new vis.Network(container, { nodes: visNodes, edges: visEdges }, {
  physics: { enabled: true, solver: "forceAtlas2Based",
    forceAtlas2Based: { gravitationalConstant: -50, centralGravity: 0.01, springLength: 100, springConstant: 0.08 },
    stabilization: { iterations: 200, enabled: true } },
  edges: { color: { inherit: true }, font: { size: 11 }, smooth: %s },
  nodes: { font: { size: 14, face: "Tahoma" }, scaling: { min: 10, max: 50 }, tooltipDelay: 200 },
  interaction: { hover: true, navigationButtons: true, keyboard: true, tooltipDelay: 200 },
  layout: { improvedLayout: true }
});

rawNodes.forEach(function(n) {
  var opt = document.createElement("option"); opt.value = n.id; opt.text = n.id;
  document.getElementById("node-select").appendChild(opt);
});

var labelsVisible = true, physicsEnabled = true;

function togglePhysics() {
  physicsEnabled = !physicsEnabled;
  network.setOptions({ physics: { enabled: physicsEnabled } });
  var btn = document.getElementById("physics-toggle");
  btn.textContent = physicsEnabled ? "Disable Physics" : "Enable Physics";
  btn.classList.toggle("active", physicsEnabled);
  if (physicsEnabled) network.startSimulation();
}
function stabilizeNetwork() { network.stabilize(100); }
function toggleLabels() {
  labelsVisible = !labelsVisible;
  var d = document.body.classList.contains("dark-mode");
  network.setOptions({
    nodes: { font: { size: labelsVisible ? 14 : 0, color: d ? "#fff" : "#000" } },
    edges: { font: { size: labelsVisible ? 12 : 0, color: d ? "#ffdd00" : "#000" } }
  });
  var btn = document.getElementById("labels-toggle");
  btn.textContent = labelsVisible ? "Hide Labels" : "Show Labels";
  btn.classList.toggle("active", labelsVisible);
}
function toggleDarkMode() {
  var isDark = document.body.classList.toggle("dark-mode");
  document.body.classList.toggle("light-mode", !isDark);
  var nc = isDark ? "#fff" : "#000", ec = isDark ? "#ffdd00" : "#000";
  network.setOptions({
    nodes: { font: { color: nc, strokeWidth: 0, size: labelsVisible ? 14 : 0 } },
    edges: { font: { color: ec, strokeWidth: isDark ? 0 : 4, size: labelsVisible ? 12 : 0 } }
  });
  visNodes.update(visNodes.getIds().map(function(id) { return { id: id, font: { color: nc, strokeWidth: 0 } }; }));
  document.getElementById("theme-toggle").textContent = isDark ? "Light Mode" : "Dark Mode";
}
function toggleStats() {
  var el = document.getElementById("stats-container"), show = el.style.display === "none";
  el.style.display = show ? "block" : "none";
  document.getElementById("stats-toggle").classList.toggle("active", show);
}
function toggleFilter() {
  var el = document.getElementById("filter-container"), show = el.style.display === "none";
  el.style.display = show ? "block" : "none";
  document.getElementById("filter-toggle").classList.toggle("active", show);
}
function selectNode(id) {
  if (!id) { resetSelection(); return; }
  network.selectNodes([id]);
  network.focus(id, { scale: 1.5, animation: { duration: 1000, easingFunction: "easeInOutQuad" } });
}
function resetSelection() {
  network.unselectAll();
  visNodes.update(visNodes.getIds().map(function(id) { return { id: id, hidden: false }; }));
  visEdges.update(visEdges.getIds().map(function(id) { return { id: id, hidden: false }; }));
}
network.once("stabilizationIterationsDone", function() { setTimeout(function() { network.fit(); }, 500); });
window.addEventListener("resize", function() { network.fit(); });
</script>
</body>
</html>`,
		stats.Nodes, stats.Edges, communities,
		stats.Nodes, stats.Edges, stats.OriginalEdges, stats.InferredEdges, communities,
		nodesJSON, edgesJSON, edgeSmoothJSON,
	)
}
