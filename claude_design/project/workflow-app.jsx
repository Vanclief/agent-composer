// Main app — canvas, pan/zoom, edges, drag, run simulation, tweaks panel.

const { useState, useEffect, useRef, useMemo, useCallback } = React;

// Tweakable defaults (host-persistable JSON)
const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "wireStyle": "bezier",
  "showGrid": true,
  "showMinimap": true,
  "density": "regular",
  "showPreview": true
}/*EDITMODE-END*/;

// ---------- Geometry helpers ----------
const NODE_W = 240;
// Header height ~ 48 + body fields. We compute port absolute positions per node.
function getPortPos(node, side, portId) {
  // Layout assumptions (must match CSS):
  // header = 48, body fields = 8 + 17*body.length + (preview? 56 : 0) + 6
  // ports area top divider, then each port row = 18 + 5gap, padding 8 top, 10 bottom
  const headerH = 48;
  const bodyFields = node.body?.length || 0;
  const hasPreview = !!(node.last?.output);
  const bodyH = 8 + bodyFields * 17 + (hasPreview ? 56 : 0) + 6;
  let y = headerH + bodyH; // start of inputs area
  // The ports section shows IN first, then OUT. Each section: 8 top pad, rows, 10 bottom pad.
  let portY = y + 8;
  if (side === 'in') {
    const idx = node.inputs.findIndex(p => p.id === portId);
    return { x: node.x, y: node.y + portY + idx * 23 + 9 };
  }
  // out — skip past inputs section
  if (node.inputs.length) {
    portY += node.inputs.length * 23 - 5 + 10; // bottom pad
    portY += 8; // out section top pad (no border, but spacing similar)
  }
  const idx = node.outputs.findIndex(p => p.id === portId);
  return { x: node.x + NODE_W, y: node.y + portY + idx * 23 + 9 };
}

function bezierPath(a, b, style='bezier') {
  if (style === 'straight') return `M ${a.x} ${a.y} L ${b.x} ${b.y}`;
  if (style === 'orthogonal') {
    const mx = (a.x + b.x) / 2;
    return `M ${a.x} ${a.y} L ${mx} ${a.y} L ${mx} ${b.y} L ${b.x} ${b.y}`;
  }
  const dx = Math.max(40, Math.abs(b.x - a.x) * 0.5);
  return `M ${a.x} ${a.y} C ${a.x + dx} ${a.y}, ${b.x - dx} ${b.y}, ${b.x} ${b.y}`;
}

// ---------- App ----------
function App() {
  const [tw, setTweak] = useTweaks(TWEAK_DEFAULTS);

  const [nodes, setNodes] = useState(INITIAL_NODES);
  const [edges] = useState(INITIAL_EDGES);
  const [selectedId, setSelectedId] = useState('summarize');
  const [runState, setRunState] = useState({}); // nodeId -> {status, progress, streamingOut}
  const [running, setRunning] = useState(false);
  const [edgeFlow, setEdgeFlow] = useState({}); // edgeId -> bool

  const [drawer, setDrawer] = useState('workflows'); // 'workflows' | 'nodes' | null
  const [selectedRunId, setSelectedRunId] = useState(RUN_HISTORY[0].id);
  const [runsMenuOpen, setRunsMenuOpen] = useState(false);
  const currentRun = RUN_HISTORY.find(r => r.id === selectedRunId) || RUN_HISTORY[0];
  const [zoom, setZoom] = useState(0.9);
  const [pan, setPan] = useState({ x: 40, y: -40 });
  const vpRef = useRef(null);
  const panState = useRef(null);
  const dragState = useRef(null);

  const selectedNode = nodes.find(n => n.id === selectedId);

  // ---------- Pan / zoom ----------
  const onCanvasMouseDown = (e) => {
    if (e.target.closest('.node')) return;
    setSelectedId(null);
    panState.current = { startX: e.clientX, startY: e.clientY, startPan: { ...pan } };
  };
  useEffect(() => {
    const move = (e) => {
      if (panState.current) {
        const dx = e.clientX - panState.current.startX;
        const dy = e.clientY - panState.current.startY;
        setPan({ x: panState.current.startPan.x + dx, y: panState.current.startPan.y + dy });
      } else if (dragState.current) {
        const dx = (e.clientX - dragState.current.startX) / zoom;
        const dy = (e.clientY - dragState.current.startY) / zoom;
        const id = dragState.current.id;
        setNodes(ns => ns.map(n => n.id === id ? { ...n, x: dragState.current.origX + dx, y: dragState.current.origY + dy } : n));
      }
    };
    const up = () => { panState.current = null; dragState.current = null; };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
    return () => { window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); };
  }, [zoom]);

  const onWheel = (e) => {
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault();
      const delta = -e.deltaY * 0.001;
      const newZoom = Math.max(0.4, Math.min(1.6, zoom + delta));
      setZoom(newZoom);
    } else {
      setPan(p => ({ x: p.x - e.deltaX, y: p.y - e.deltaY }));
    }
  };

  const onNodeDragStart = (e, id) => {
    if (e.target.closest('.dot')) return; // ignore port grabs for now
    e.stopPropagation();
    const n = nodes.find(x => x.id === id);
    dragState.current = { id, startX: e.clientX, startY: e.clientY, origX: n.x, origY: n.y };
    setSelectedId(id);
  };

  // ---------- Run simulation ----------
  const startRun = useCallback(() => {
    if (running) {
      setRunning(false);
      setRunState({});
      setEdgeFlow({});
      return;
    }
    setRunning(true);
    setRunState({});
    setEdgeFlow({});
    // sequential by topological order matching INITIAL_NODES order
    const order = ['trigger', 'plan', 'search', 'scrape', 'summarize', 'writer'];
    const durations = { trigger: 200, plan: 1500, search: 1800, scrape: 2200, summarize: 2400, writer: 2000 };
    let i = 0;
    let cancelled = false;

    const step = () => {
      if (cancelled || i >= order.length) {
        setRunning(false);
        return;
      }
      const id = order[i];
      // Activate edges that lead INTO this node
      setEdgeFlow(prev => {
        const next = { ...prev };
        edges.forEach(e => { if (e.to === id) next[e.id] = true; });
        return next;
      });
      setRunState(prev => ({ ...prev, [id]: { status: 'run', streamingOut: {} } }));

      // Stream tokens for LLM-like nodes
      const node = nodes.find(n => n.id === id);
      const outs = node.outputs;
      const targetOut = outs[0]?.id;
      const targetText = (() => {
        const v = node.last?.output?.[targetOut];
        if (typeof v === 'string') return v;
        return null;
      })();
      let streamI = 0;
      const streamTimer = targetText ? setInterval(() => {
        streamI = Math.min(targetText.length, streamI + Math.max(2, Math.floor(targetText.length / 25)));
        setRunState(prev => ({ ...prev, [id]: { status: 'run', streamingOut: { [targetOut]: targetText.slice(0, streamI) } } }));
        if (streamI >= targetText.length) clearInterval(streamTimer);
      }, durations[id] / 30) : null;

      setTimeout(() => {
        if (cancelled) return;
        if (streamTimer) clearInterval(streamTimer);
        setRunState(prev => ({ ...prev, [id]: { status: 'ok' } }));
        setEdgeFlow(prev => {
          const next = { ...prev };
          edges.forEach(e => { if (e.to === id) next[e.id] = false; });
          return next;
        });
        i++;
        step();
      }, durations[id]);
    };
    step();
  }, [running, edges, nodes]);

  // ---------- Edges SVG ----------
  const wirePaths = useMemo(() => {
    const OFFSET = 4000;
    return edges.map(e => {
      const fromN = nodes.find(n => n.id === e.from);
      const toN = nodes.find(n => n.id === e.to);
      if (!fromN || !toN) return null;
      const a = getPortPos(fromN, 'out', e.fromPort);
      const b = getPortPos(toN, 'in', e.toPort);
      const port = fromN.outputs.find(p => p.id === e.fromPort);
      const color = `var(--t-${port.type})`;
      const path = bezierPath(
        { x: a.x + OFFSET, y: a.y + OFFSET },
        { x: b.x + OFFSET, y: b.y + OFFSET },
        tw.wireStyle
      );
      return { id: e.id, path, color, flow: !!edgeFlow[e.id] };
    }).filter(Boolean);
  }, [nodes, edges, edgeFlow, tw.wireStyle]);

  // ---------- Run progress strip ----------
  const order = ['trigger', 'plan', 'search', 'scrape', 'summarize', 'writer'];

  // ---------- Render ----------
  return (
    <div className="app">
      {/* Topbar */}
      <div className="topbar">
        <div className="brand">
          <div className="logo">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" style={{position:'absolute',inset:0,margin:'auto',color:'var(--ink)'}}>
              <circle cx="3.5" cy="3.5" r="1.6"/>
              <circle cx="10.5" cy="7" r="1.6"/>
              <circle cx="3.5" cy="10.5" r="1.6"/>
              <path d="M5 4.2 L9 6.3 M5 9.8 L9 7.7"/>
            </svg>
          </div>
          <b>agc</b>
        </div>
        <div className="crumbs" style={{paddingLeft:6}}>
          <span>Workspace</span>
          <span className="sep">/</span>
          <span>Workflows</span>
          <span className="sep">/</span>
          <span className="cur">Research agent</span>
          <span className="pill">v12</span>
          <span style={{color:'var(--ink-3)', fontSize:11, marginLeft:4}}>· edited 2m ago</span>
        </div>
        <div className="spacer"/>
        <span className="runmenu-anchor">
          <button className="ghostbtn" onClick={() => setRunsMenuOpen(o=>!o)}>
            <Ico.History/> {currentRun.id}
            <svg className="chev" width="9" height="9" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M2.5 4 L5 6.5 L7.5 4"/></svg>
          </button>
          {runsMenuOpen && (
            <RunMenu runs={RUN_HISTORY} currentId={currentRun.id} top={36} align="right"
              onPick={(id) => { setRunsMenuOpen(false); setSelectedRunId(id); }}
              onClose={() => setRunsMenuOpen(false)}/>
          )}
        </span>
        <button className="ghostbtn"><Ico.Share/> Share</button>
        <div className="avatars" style={{margin:'0 6px 0 4px'}}>
          <div className="avatar" style={{background:'oklch(0.62 0.16 250)'}}>JM</div>
          <div className="avatar" style={{background:'oklch(0.65 0.14 145)'}}>AS</div>
          <div className="avatar" style={{background:'oklch(0.68 0.14 50)'}}>RK</div>
        </div>
        <button className={'runbtn ' + (running ? 'running' : '')} onClick={startRun}>
          {running ? (<><Ico.Stop/> Stop</>) : (<><Ico.Play/> Run workflow</>)}
        </button>
      </div>

      {/* Icon rail */}
      <aside className="rail">
        <button className={'rail-btn ' + (drawer==='workflows'?'active':'')} onClick={() => setDrawer('workflows')} title="Workflows">
          <Ico.Stack s={16}/>
          {WORKFLOWS.some(w => w.running) && <span className="badge"/>}
        </button>
        <button className={'rail-btn ' + (drawer==='nodes'?'active':'')} onClick={() => setDrawer('nodes')} title="Node library">
          <Ico.Blocks s={16}/>
        </button>
        <button className="rail-btn" title="Triggers"><Ico.Bolt s={16}/></button>
        <div className="rail-divider"/>
        <button className="rail-btn" title="History"><Ico.History/></button>
        <div style={{flex:1}}/>
        <button className="rail-btn" title="Settings"><Ico.Cog s={16}/></button>
      </aside>

      {/* Drawer */}
      <aside className="left scrollnice">
        {drawer === 'workflows' && (
          <>
            <div className="drawer-head">
              <h3>Workflows</h3>
              <button className="iconbtn" style={{width:24,height:24,border:0}} title="New workflow"><Ico.Plus s={13}/></button>
            </div>
            <div className="drawer-search"><input className="input" placeholder="Search workflows…"/></div>
            <div className="ws-list" style={{paddingTop:4}}>
              {WORKFLOWS.map(w => (
                <div key={w.id} className={'ws-item ' + (w.active ? 'active' : '') + (w.running ? ' run' : '')}>
                  <span className="ws-dot"/>
                  <span style={{flex:1, minWidth:0, whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis'}}>{w.name}</span>
                  <span className="ws-meta">{w.meta}</span>
                </div>
              ))}
            </div>
            <div className="divider"/>
            <div style={{padding:'10px 14px', fontSize:11, color:'var(--ink-3)', lineHeight:1.5}}>
              5 workflows · 1 running<br/>
              <span style={{fontFamily:'JetBrains Mono, monospace'}}>last sync · 14:32</span>
            </div>
          </>
        )}
        {drawer === 'nodes' && (
          <>
            <div className="drawer-head">
              <h3>Node library</h3>
              <span className="meta">drag → canvas</span>
            </div>
            <div className="drawer-search"><input className="input" placeholder="Search nodes…"/></div>
            <div className="lib" style={{paddingTop:4}}>
              {NODE_LIBRARY.map(sec => (
                <React.Fragment key={sec.section}>
                  <div style={{fontSize:10.5, fontWeight:600, letterSpacing:'0.05em', textTransform:'uppercase', color:'var(--ink-3)', padding:'10px 8px 4px'}}>
                    {sec.section}
                  </div>
                  {sec.items.map(it => {
                    const v = KIND_VISUAL[it.kind];
                    return (
                      <div key={it.name} className="lib-item" draggable>
                        <div className="lib-icon" style={{background:v.bg, color:v.fg}}>
                          <KindIcon kind={it.kind} s={12}/>
                        </div>
                        <div style={{flex:1, minWidth:0}}>
                          <div style={{fontWeight:500}}>{it.name}</div>
                          <div style={{fontSize:11, color:'var(--ink-3)'}}>{it.sub}</div>
                        </div>
                      </div>
                    );
                  })}
                </React.Fragment>
              ))}
            </div>
          </>
        )}
      </aside>

      {/* Canvas */}
      <main className="center" ref={vpRef} onMouseDown={onCanvasMouseDown} onWheel={onWheel}>
        {tw.showGrid && <div className="canvas-grid" style={{
          backgroundPosition: `${pan.x}px ${pan.y}px`,
          backgroundSize: `${22*zoom}px ${22*zoom}px`,
        }}/>}

        <div className="canvas-vp">
          <div className="canvas-world" style={{ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`, transformOrigin: '0 0' }}>
            {/* Wires */}
            <svg className="wires" xmlns="http://www.w3.org/2000/svg">
              {wirePaths.map(w => (
                <g key={w.id}>
                  <path className="wire-hit" d={w.path}/>
                  {/* base wire (faded behind flow) */}
                  <path className="wire" d={w.path} stroke={w.color} opacity={w.flow ? 0.25 : 0.55}/>
                  {w.flow && <path className="wire flow" d={w.path} stroke={w.color} opacity={0.95}/>}
                </g>
              ))}
            </svg>

            {/* Nodes */}
            {nodes.map(n => (
              <WorkflowNode
                key={n.id}
                node={n}
                selected={selectedId === n.id}
                onSelect={setSelectedId}
                onDragStart={onNodeDragStart}
                runtime={runState[n.id]}
                currentRun={currentRun}
                allRuns={RUN_HISTORY}
                onSelectRun={setSelectedRunId}
              />
            ))}
          </div>
        </div>

        {/* Canvas tools */}
        <div className="canvas-tools">
          <div className="ct-grp">
            <button className="iconbtn" onClick={() => setZoom(z => Math.min(1.6, z + 0.1))}><Ico.ZoomIn/></button>
            <button className="iconbtn" onClick={() => setZoom(z => Math.max(0.4, z - 0.1))}><Ico.ZoomOut/></button>
            <button className="iconbtn" onClick={() => { setZoom(0.9); setPan({ x: 40, y: -40 }); }}><Ico.Fit/></button>
          </div>
          <div className="ct-grp"><div className="zlbl">{Math.round(zoom*100)}%</div></div>
        </div>

        {/* Minimap */}
        {tw.showMinimap && <Minimap nodes={nodes} runState={runState}/>}

        {/* Run strip */}
        <div className="run-strip">
          <span className="lbl">Run</span>
          <span className="runmenu-anchor">
            <span className="pill clickable" style={{height:20}} onClick={() => setRunsMenuOpen(o=>!o)}>
              <span className="dot" style={{background: currentRun.status === 'err' ? 'var(--st-err)' : 'var(--st-ok)'}}/>
              {currentRun.id}
              <svg className="chev" width="9" height="9" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M2.5 4 L5 6.5 L7.5 4"/></svg>
            </span>
          </span>
          <span style={{fontSize:11, color:'var(--ink-3)'}}>{currentRun.whenAbs} · {currentRun.when}</span>
          <div className="run-bar">
            {order.map(id => {
              const rs = runState[id];
              const snap = currentRun.nodes?.[id];
              const cls = rs?.status === 'run' ? 'run'
                : rs?.status === 'ok' ? 'done'
                : (snap?.status === 'ok' ? 'done' : (snap?.status === 'err' ? 'err' : ''));
              return <div key={id} className={'rb ' + cls} title={`${id} · ${snap?.status || 'idle'}`}/>;
            })}
          </div>
          <span style={{fontFamily:'JetBrains Mono, monospace', fontSize:11.5, color:'var(--ink-2)'}}>
            {running ? '⋯ running' : `${(currentRun.duration/1000).toFixed(1)}s · ${currentRun.tokens.toLocaleString()} tok · $${currentRun.cost.toFixed(3)}`}
          </span>
          <button className="ghostbtn" style={{height:26, padding:'0 8px', fontSize:12}}>View trace →</button>
        </div>
      </main>

      {/* Right inspector */}
      <aside className="right">
        <Inspector
          node={selectedNode}
          runtime={runState[selectedId]}
          currentRun={currentRun}
          allRuns={RUN_HISTORY}
          onSelectRun={setSelectedRunId}
        />
      </aside>

      {/* Tweaks */}
      <TweaksPanel>
        <TweakSection label="Canvas"/>
        <TweakRadio label="Wire style" value={tw.wireStyle} options={['bezier','orthogonal','straight']} onChange={v => setTweak('wireStyle', v)}/>
        <TweakToggle label="Grid" value={tw.showGrid} onChange={v => setTweak('showGrid', v)}/>
        <TweakToggle label="Minimap" value={tw.showMinimap} onChange={v => setTweak('showMinimap', v)}/>
        <TweakSection label="Nodes"/>
        <TweakToggle label="Last-run preview" value={tw.showPreview} onChange={v => setTweak('showPreview', v)}/>
        <TweakRadio label="Density" value={tw.density} options={['compact','regular']} onChange={v => setTweak('density', v)}/>
      </TweaksPanel>
    </div>
  );
}

function Minimap({ nodes, runState }) {
  // bounding box of all nodes in world space
  const minX = Math.min(...nodes.map(n => n.x)) - 20;
  const minY = Math.min(...nodes.map(n => n.y)) - 20;
  const maxX = Math.max(...nodes.map(n => n.x + NODE_W)) + 20;
  const maxY = Math.max(...nodes.map(n => n.y + 280)) + 20;
  const w = maxX - minX, h = maxY - minY;
  const sx = 160 / w, sy = 100 / h;
  const s = Math.min(sx, sy);
  return (
    <div className="minimap">
      {nodes.map(n => {
        const rs = runState[n.id];
        const cls = rs?.status === 'run' ? 'run' : (rs?.status === 'ok' ? 'ok' : '');
        return <div key={n.id} className={'mm-node ' + cls} style={{
          left: (n.x - minX) * s, top: (n.y - minY) * s,
          width: NODE_W * s, height: 200 * s,
        }}/>;
      })}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App/>);
