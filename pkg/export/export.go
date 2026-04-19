package export

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sjhorn/graphify/pkg/cluster"
	"github.com/sjhorn/graphify/pkg/graph"
)

// ToJSON exports the graph to JSON format.
func ToJSON(g *graph.Graph, result *cluster.ClusterResult, outPath string) error {
	nodes := g.Nodes()
	edges := g.Edges()

	// Build community map
	nodeCommunity := result.NodeCommunity

	// Build nodes array
	nodesJSON := make([]map[string]interface{}, len(nodes))
	for i, node := range nodes {
		community := 0
		if cid, ok := nodeCommunity[node.ID]; ok {
			community = cid
		}
		nodesJSON[i] = map[string]interface{}{
			"id":        node.ID,
			"label":     node.Label,
			"type":      node.Type,
			"file":      node.File,
			"community": community,
		}
	}

	// Build links array
	linksJSON := make([]map[string]interface{}, len(edges))
	for i, edge := range edges {
		linksJSON[i] = map[string]interface{}{
			"source":   edge.Source,
			"target":   edge.Target,
			"relation": edge.Relation,
		}
	}

	output := map[string]interface{}{
		"nodes": nodesJSON,
		"links": linksJSON,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0644)
}

// ToCypher exports the graph to Neo4j Cypher format.
func ToCypher(g *graph.Graph, outPath string) error {
	var lines []string

	// Create nodes
	for _, node := range g.Nodes() {
		label := sanitizeLabel(node.Label)
		line := fmt.Sprintf("MERGE (n:Node {id: '%s', label: '%s'})", node.ID, label)
		lines = append(lines, line)
	}

	// Create edges
	for _, edge := range g.Edges() {
		rel := sanitizeLabel(edge.Relation)
		line := fmt.Sprintf("MATCH (a:Node), (b:Node) WHERE a.id = '%s' AND b.id = '%s' MERGE (a)-[r:%s]->(b)",
			edge.Source, edge.Target, rel)
		lines = append(lines, line)
	}

	return os.WriteFile(outPath, []byte(strings.Join(lines, "\n")), 0644)
}

// ToGraphML exports the graph to GraphML format.
func ToGraphML(g *graph.Graph, result *cluster.ClusterResult, outPath string) error {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">`)
	sb.WriteString(`<key id="community" for="node" attr.name="community" attr.type="int"/>`)
	sb.WriteString(`<graph id="G" edgedefault="undirected">`)

	// Build community map
	nodeCommunity := result.NodeCommunity

	// Write nodes
	for _, node := range g.Nodes() {
		cid := 0
		if c, ok := nodeCommunity[node.ID]; ok {
			cid = c
		}
		sb.WriteString(fmt.Sprintf(`<node id="%s">`, node.ID))
		sb.WriteString(fmt.Sprintf(`<data key="community">%d</data>`, cid))
		sb.WriteString("</node>")
	}

	// Write edges
	for _, edge := range g.Edges() {
		sb.WriteString(fmt.Sprintf(`<edge source="%s" target="%s"/>`, edge.Source, edge.Target))
	}

	sb.WriteString("</graph>")
	sb.WriteString("</graphml>")

	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}

// ToHTML exports the graph to an interactive HTML visualization.
func ToHTML(g *graph.Graph, result *cluster.ClusterResult, outPath string, communityLabels map[int]string) error {
	nodes := g.Nodes()
	edges := g.Edges()

	// Build community map
	nodeCommunity := result.NodeCommunity

	// Assign colors to communities
	communityColors := []string{
		"#4E79A7", "#F28E2B", "#E15759", "#76B7B2", "#59A14F",
		"#EDC948", "#B07AA1", "#FF9DA7", "#9C755F", "#BAB0AC",
	}

	communityColorMap := make(map[int]string)
	for i := range result.Communities {
		communityColorMap[i] = communityColors[i%len(communityColors)]
	}

	// Build nodes
	type nodeData struct {
		ID        string `json:"id"`
		Label     string `json:"label"`
		Community int    `json:"community"`
		Color     string `json:"color"`
	}
	nodeList := make([]nodeData, len(nodes))
	for i, node := range nodes {
		cid := 0
		if c, ok := nodeCommunity[node.ID]; ok {
			cid = c
		}
		nodeList[i] = nodeData{
			ID:        node.ID,
			Label:     node.Label,
			Community: cid,
			Color:     communityColorMap[cid],
		}
	}

	// Build edges
	type edgeData struct {
		Source string `json:"from"`
		Target string `json:"to"`
	}
	edgeList := make([]edgeData, len(edges))
	for i, edge := range edges {
		edgeList[i] = edgeData{
			Source: edge.Source,
			Target: edge.Target,
		}
	}

	// Generate legend items
	var legendItems []string
	for cid := range result.Communities {
		label := fmt.Sprintf("Community %d", cid)
		if communityLabels != nil {
			if l, ok := communityLabels[cid]; ok {
				label = l
			}
		}
		color := communityColorMap[cid]
		legendItems = append(legendItems, fmt.Sprintf(
			`<div class="legend-item" data-community="%d"><span class="legend-dot" style="background:%s"></span><span class="legend-label">%s</span><span class="legend-count">%d nodes</span></div>`,
			cid, color, label, len(result.Communities[cid])))
	}

	// Sort legend items
	sort.Slice(legendItems, func(i, j int) bool {
		return i < j
	})

	// Generate nodes JSON
	nodesJSON, _ := json.Marshal(nodeList)
	edgesJSON, _ := json.Marshal(edgeList)

	// Build HTML
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>Graph Visualization</title>
  <script src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
  %s
</head>
<body>
  <div id="graph"></div>
  <div id="sidebar">
    <div id="search-wrap">
      <input type="text" id="search" placeholder="Search nodes...">
      <div id="search-results"></div>
    </div>
    <div id="info-panel">
      <h3>Node Info</h3>
      <div id="info-content"><span class="empty">Select a node to see details</span></div>
      <h3>Neighbors</h3>
      <div id="neighbors-list"></div>
    </div>
    <div id="legend-wrap">
      <h3>Communities</h3>
      %s
    </div>
    <div id="stats">Nodes: %d | Edges: %d</div>
  </div>
  <script>
    const RAW_NODES = %s;
    const RAW_EDGES = %s;

    const nodes = new vis.DataSet(RAW_NODES);
    const edges = new vis.DataSet(RAW_EDGES);

    const container = document.getElementById('graph');
    const data = { nodes, edges };
    const options = {
      nodes: { shape: 'dot', size: 16, font: { size: 14, color: '#ffffff' }, borderWidth: 2, shadow: true },
      edges: { width: 1, color: { color: 'rgba(255,255,255,0.3)', highlight: '#888' }, smooth: { type: 'continuous' } },
      physics: { stabilization: false, barnesHut: { gravitationalConstant: -3000, springConstant: 0.04, springLength: 95 } },
      interaction: { hover: true }
    };
    const network = new vis.Network(container, data, options);

    const searchInput = document.getElementById('search');
    const searchResults = document.getElementById('search-results');

    searchInput.addEventListener('input', function(e) {
      const query = e.target.value.toLowerCase();
      if (query.length < 2) { searchResults.style.display = 'none'; return; }
      const matches = RAW_NODES.filter(function(n) { return n.label.toLowerCase().includes(query); });
      searchResults.innerHTML = matches.map(function(n) {
        return '<div class="search-item" data-id="' + n.id + '">' + htmlEscape(n.label) + '</div>';
      }).join('');
      searchResults.style.display = 'block';
    });

    searchResults.addEventListener('click', function(e) {
      if (e.target.classList.contains('search-item')) {
        const nodeId = e.target.dataset.id;
        network.selectNodes([nodeId]);
        network.focus(nodeId, { animation: true });
      }
    });

    network.on('selectNode', function(params) {
      const nodeId = params.nodes[0];
      const node = RAW_NODES.find(function(n) { return n.id === nodeId; });
      if (!node) return;
      const infoContent = document.getElementById('info-content');
      infoContent.innerHTML = '<div class="field"><b>ID:</b> ' + htmlEscape(node.id) + '</div><div class="field"><b>Label:</b> ' + htmlEscape(node.label) + '</div><div class="field"><b>Community:</b> ' + String(node.community) + '</div>';
      const neighbors = network.getConnectedNodes(nodeId);
      const neighborsList = document.getElementById('neighbors-list');
      neighborsList.innerHTML = neighbors.map(function(nid) {
        const neighbor = RAW_NODES.find(function(n) { return n.id === nid; });
        return '<div class="neighbor-link" data-id="' + nid + '">' + htmlEscape(neighbor ? neighbor.label : nid) + '</div>';
      }).join('');
    });

    document.querySelectorAll('.legend-item').forEach(function(item) {
      item.addEventListener('click', function() {
        const community = parseInt(item.dataset.community);
        const nodeIds = RAW_NODES.filter(function(n) { return n.community === community; }).map(function(n) { return n.id; });
        network.selectNodes(nodeIds);
        network.fit({ nodes: nodeIds, animation: true });
      });
    });

    function htmlEscape(str) {
      var div = document.createElement('div');
      div.textContent = str;
      return div.innerHTML;
    }
  </script>
</body>
</html>`, _htmlStyles(), strings.Join(legendItems, "\n"), len(nodes), len(edges), string(nodesJSON), string(edgesJSON))

	return os.WriteFile(outPath, []byte(htmlContent), 0644)
}

// sanitizeLabel escapes special characters for use in labels.
func sanitizeLabel(label string) string {
	label = strings.ReplaceAll(label, "'", "\\'")
	label = strings.ReplaceAll(label, "\n", " ")
	return label
}

func _htmlStyles() string {
	return `<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #0f0f1a; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; display: flex; height: 100vh; overflow: hidden; }
  #graph { flex: 1; }
  #sidebar { width: 280px; background: #1a1a2e; border-left: 1px solid #2a2a4e; display: flex; flex-direction: column; overflow: hidden; }
  #search-wrap { padding: 12px; border-bottom: 1px solid #2a2a4e; }
  #search { width: 100%; background: #0f0f1a; border: 1px solid #3a3a5e; color: #e0e0e0; padding: 7px 10px; border-radius: 6px; font-size: 13px; outline: none; }
  #search:focus { border-color: #4E79A7; }
  #search-results { max-height: 140px; overflow-y: auto; padding: 4px 12px; border-bottom: 1px solid #2a2a4e; display: none; }
  .search-item { padding: 4px 6px; cursor: pointer; border-radius: 4px; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .search-item:hover { background: #2a2a4e; }
  #info-panel { padding: 14px; border-bottom: 1px solid #2a2a4e; min-height: 140px; }
  #info-panel h3 { font-size: 13px; color: #aaa; margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.05em; }
  #info-content { font-size: 13px; color: #ccc; line-height: 1.6; }
  #info-content .field { margin-bottom: 5px; }
  #info-content .empty { color: #555; font-style: italic; }
  .neighbor-link { display: block; padding: 2px 6px; margin: 2px 0; border-radius: 3px; cursor: pointer; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; border-left: 3px solid #333; }
  .neighbor-link:hover { background: #2a2a4e; }
  #neighbors-list { max-height: 160px; overflow-y: auto; margin-top: 4px; }
  #legend-wrap { flex: 1; overflow-y: auto; padding: 12px; }
  #legend-wrap h3 { font-size: 13px; color: #aaa; margin-bottom: 10px; text-transform: uppercase; letter-spacing: 0.05em; }
  .legend-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; cursor: pointer; border-radius: 4px; font-size: 12px; }
  .legend-item:hover { background: #2a2a4e; padding-left: 4px; }
  .legend-dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
  .legend-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .legend-count { color: #666; font-size: 11px; }
  #stats { padding: 10px 14px; border-top: 1px solid #2a2a4e; font-size: 11px; color: #555; }
</style>`
}
