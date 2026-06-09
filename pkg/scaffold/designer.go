package scaffold

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DesignerOptions 控制本地可视化脚手架设计器。
type DesignerOptions struct {
	RootDir string
	Addr    string
}

type designerSchemaRequest struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
	Schema string `json:"schema"`
}

type designerPlanResponse struct {
	Path string `json:"path"`
}

type designerGenerateRequest struct {
	Plan     GenerationPlan `json:"plan"`
	RunProto bool           `json:"run_proto"`
	RunTidy  bool           `json:"run_tidy"`
}

// StartDesigner 启动只监听本机地址的可视化生成器。
func StartDesigner(opts DesignerOptions) error {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:6060"
	}
	root, err := serviceRoot(opts.RootDir)
	if err != nil {
		return err
	}
	opts.RootDir = root
	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Fprintf(os.Stdout, "designer listening at http://%s\n", listener.Addr().String())
	return http.Serve(listener, newDesignerMux(opts))
}

func newDesignerMux(opts DesignerOptions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(designerHTML))
	})
	mux.HandleFunc("/api/schema", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeDesignerError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req designerSchemaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		snapshot, err := InspectDatabaseSchema(DatabaseConnection{Driver: req.Driver, DSN: req.DSN}, req.Schema)
		if err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDesignerJSON(w, snapshot)
	})
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeDesignerError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var plan GenerationPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		path := defaultPlanPath(opts.RootDir, plan.ServiceName)
		if err := SaveGenerationPlan(path, plan); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDesignerJSON(w, designerPlanResponse{Path: path})
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeDesignerError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req designerGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		path := defaultPlanPath(opts.RootDir, req.Plan.ServiceName)
		if err := SaveGenerationPlan(path, req.Plan); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := AddService(ServiceOptions{
			RootDir:    opts.RootDir,
			Name:       req.Plan.ServiceName,
			PlanPath:   path,
			RunProto:   req.RunProto,
			RunTidy:    req.RunTidy,
			TableName:  req.Plan.RootTable,
			SchemaName: req.Plan.Schema,
		}); err != nil {
			writeDesignerError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDesignerJSON(w, designerPlanResponse{Path: path})
	})
	return mux
}

func writeDesignerJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeDesignerError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func resolvePlanPath(root string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, strings.TrimSpace(path))
}

const designerHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>bw-cli Designer</title>
  <style>
    :root {
      --ink: #17211b;
      --muted: #5d6a61;
      --line: #d5ddd5;
      --paper: #f8faf5;
      --panel: #ffffff;
      --accent: #0f7b5c;
      --accent-2: #c4492d;
      --shadow: 0 18px 55px rgba(23,33,27,.12);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      color: var(--ink);
      background: radial-gradient(circle at top left, #dceee2, transparent 34rem), var(--paper);
      font: 15px/1.5 ui-serif, Georgia, Cambria, "Times New Roman", serif;
    }
    main { max-width: 1180px; margin: 0 auto; padding: 32px 20px 56px; }
    header { display: grid; grid-template-columns: 1.2fr .8fr; gap: 22px; align-items: end; margin-bottom: 24px; }
    h1 { margin: 0; font-size: clamp(34px, 5vw, 68px); line-height: .92; letter-spacing: 0; }
    h2 { margin: 0 0 12px; font-size: 18px; }
    p { margin: 0; color: var(--muted); }
    .grid { display: grid; grid-template-columns: 360px 1fr; gap: 18px; align-items: start; }
    section, aside {
      background: rgba(255,255,255,.84);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      padding: 18px;
    }
    label { display: block; margin: 12px 0 6px; color: var(--muted); font-size: 13px; }
    input, select, textarea {
      width: 100%;
      min-height: 40px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      color: var(--ink);
      padding: 9px 10px;
      font: 14px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    button {
      min-height: 40px;
      border: 0;
      border-radius: 6px;
      background: var(--accent);
      color: white;
      padding: 9px 13px;
      cursor: pointer;
      font-weight: 700;
    }
    button.secondary { background: var(--ink); }
    button.warn { background: var(--accent-2); }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
    .actions { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 14px; }
    .tables { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 12px; }
    .table {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px;
      background: var(--panel);
    }
    .table strong { display: flex; gap: 8px; align-items: center; }
    .columns { margin-top: 8px; display: grid; gap: 4px; color: var(--muted); font: 12px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .relation { display: grid; grid-template-columns: 1fr 1fr 1fr 1fr 130px 1fr; gap: 8px; margin-top: 8px; }
    pre {
      min-height: 190px;
      white-space: pre-wrap;
      overflow: auto;
      border: 1px dashed var(--line);
      border-radius: 8px;
      padding: 12px;
      background: #fbfcf8;
      color: #27362d;
    }
    .status { margin-top: 12px; color: var(--accent-2); font-weight: 700; }
    @media (max-width: 860px) {
      header, .grid { grid-template-columns: 1fr; }
      .relation { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
<main>
  <header>
    <div>
      <h1>Schema Composer</h1>
      <p>连接数据库，选择单表或多表关系，生成 bw-cli 服务计划并可直接写入脚手架代码。</p>
    </div>
    <aside>
      <label>服务名</label>
      <input id="serviceName" value="product">
      <label>主表</label>
      <select id="rootTable"></select>
    </aside>
  </header>
  <div class="grid">
    <section>
      <h2>数据库连接</h2>
      <div class="row">
        <div>
          <label>驱动</label>
          <select id="driver"><option>mysql</option><option>postgres</option><option>sqlite</option></select>
        </div>
        <div>
          <label>Schema</label>
          <input id="schema" placeholder="public / database">
        </div>
      </div>
      <label>DSN</label>
      <textarea id="dsn" rows="4">账号:密码@tcp(服务器IP:3306)/数据库?charset=utf8mb4&parseTime=True&loc=Local</textarea>
      <div class="actions">
        <button onclick="loadSchema()">读取表结构</button>
        <button class="secondary" onclick="buildPlan()">预览计划</button>
      </div>
      <p class="status" id="status"></p>
    </section>
    <section>
      <h2>表选择</h2>
      <div class="tables" id="tables"></div>
      <h2 style="margin-top:18px">关系配置</h2>
      <div id="relations"></div>
      <div class="actions">
        <button class="secondary" onclick="addRelation()">添加关系</button>
        <button onclick="savePlan()">保存计划</button>
        <button class="warn" onclick="generate()">生成代码</button>
      </div>
      <h2 style="margin-top:18px">计划预览</h2>
      <pre id="preview">{}</pre>
    </section>
  </div>
</main>
<script>
let schemaSnapshot = { tables: [], foreign_keys: [] };

function setStatus(text) { document.getElementById('status').textContent = text || ''; }
function selectedTables() {
  return [...document.querySelectorAll('[data-table]:checked')].map(input => input.value);
}
function tableOptions() {
  return schemaSnapshot.tables.map(t => '<option value="' + t.name + '">' + t.name + '</option>').join('');
}
async function loadSchema() {
  setStatus('读取中...');
  const response = await fetch('/api/schema', {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({
      driver: document.getElementById('driver').value,
      dsn: document.getElementById('dsn').value,
      schema: document.getElementById('schema').value
    })
  });
  const data = await response.json();
  if (!response.ok) { setStatus(data.error || '读取失败'); return; }
  schemaSnapshot = data;
  renderTables();
  renderRelations(data.foreign_keys || []);
  setStatus('已读取 ' + data.tables.length + ' 张表');
}
function renderTables() {
  const root = document.getElementById('rootTable');
  root.innerHTML = tableOptions();
  document.getElementById('tables').innerHTML = schemaSnapshot.tables.map(table => {
    const columns = table.columns.map(c => '<span>' + c.name + ' · ' + c.type + (c.primary_key ? ' · PK' : '') + '</span>').join('');
    return '<div class="table"><strong><input type="checkbox" data-table value="' + table.name + '" checked> ' + table.name + '</strong><div class="columns">' + columns + '</div></div>';
  }).join('');
}
function renderRelations(relations) {
  document.getElementById('relations').innerHTML = '';
  relations.forEach(rel => addRelation(rel));
}
function addRelation(rel = {}) {
  const row = document.createElement('div');
  row.className = 'relation';
  row.innerHTML = '<select class="fromTable">' + tableOptions() + '</select><input class="fromColumn" placeholder="from column"><select class="toTable">' + tableOptions() + '</select><input class="toColumn" placeholder="to column"><select class="relType"><option value="one_to_many">一对多</option><option value="one_to_one">一对一</option><option value="many_to_one">多对一</option><option value="many_to_many">多对多</option></select><input class="joinTable" placeholder="join table">';
  document.getElementById('relations').appendChild(row);
  row.querySelector('.fromTable').value = rel.from_table || '';
  row.querySelector('.fromColumn').value = rel.from_column || '';
  row.querySelector('.toTable').value = rel.to_table || '';
  row.querySelector('.toColumn').value = rel.to_column || '';
  row.querySelector('.relType').value = rel.type || 'one_to_many';
  row.querySelector('.joinTable').value = rel.join_table || '';
}
function buildPlan() {
  const relationships = [...document.querySelectorAll('.relation')].map(row => ({
    type: row.querySelector('.relType').value,
    from_table: row.querySelector('.fromTable').value,
    from_column: row.querySelector('.fromColumn').value,
    to_table: row.querySelector('.toTable').value,
    to_column: row.querySelector('.toColumn').value,
    join_table: row.querySelector('.joinTable').value
  })).filter(rel => rel.from_table && rel.from_column && rel.to_table && rel.to_column);
  const plan = {
    service_name: document.getElementById('serviceName').value,
    root_table: document.getElementById('rootTable').value,
    schema: document.getElementById('schema').value,
    tables: selectedTables(),
    relationships
  };
  document.getElementById('preview').textContent = JSON.stringify(plan, null, 2);
  return plan;
}
async function savePlan() {
  const response = await fetch('/api/plan', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(buildPlan()) });
  const data = await response.json();
  setStatus(response.ok ? '计划已保存：' + data.path : data.error);
}
async function generate() {
  const response = await fetch('/api/generate', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({ plan: buildPlan(), run_proto: true, run_tidy: false }) });
  const data = await response.json();
  setStatus(response.ok ? '代码已生成，计划：' + data.path : data.error);
}
</script>
</body>
</html>`
