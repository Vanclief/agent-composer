// Inspector — right panel. Tabs: Inspector (live last-run I/O) and Config.

function Inspector({ node, runtime, onUpdate, currentRun, allRuns, onSelectRun }) {
  const [tab, setTab] = React.useState('insp');
  React.useEffect(() => { setTab('insp'); }, [node?.id]);

  if (!node) {
    return (
      <div className="insp">
        <div className="insp-empty">
          <div style={{fontSize:32, marginBottom:8, color:'var(--ink-4)'}}>
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round">
              <rect x="6" y="9" width="14" height="10" rx="2"/>
              <rect x="20" y="21" width="14" height="10" rx="2"/>
              <path d="M20 14 H30 M30 14 V19"/>
            </svg>
          </div>
          <div style={{color:'var(--ink-2)', fontWeight:500, marginBottom:4}}>Nothing selected</div>
          <div>Click a node to inspect its config<br/>and live last-run I/O.</div>
        </div>
      </div>
    );
  }

  const v = KIND_VISUAL[node.kind];
  const snap = currentRun?.nodes?.[node.id];
  const status = runtime?.status || snap?.status || 'idle';
  const tokens = snap?.tokens;
  const ms = snap?.ms;
  const [menuOpen, setMenuOpen] = React.useState(false);

  return (
    <div className="insp">
      <div className="insp-head">
        <div className="icon" style={{background:v.bg, color:v.fg}}>
          <KindIcon kind={node.kind} s={15}/>
        </div>
        <div style={{flex:1, minWidth:0}}>
          <h3>{node.name}</h3>
          <div className="sub">{node.kind} · {node.id}</div>
        </div>
        <span className="runmenu-anchor">
          <StatusPill status={status} tokens={tokens} ms={ms} runId={currentRun?.id}
            onClick={status !== 'run' ? () => setMenuOpen(o=>!o) : undefined}/>
          {menuOpen && (
            <RunMenu runs={allRuns} currentId={currentRun?.id} nodeId={node.id} align="right" top={28}
              onPick={(id)=>{setMenuOpen(false); onSelectRun(id);}}
              onClose={()=>setMenuOpen(false)}/>
          )}
        </span>
      </div>

      <div className="tabs">
        <div className={'tab ' + (tab==='insp'?'active':'')} onClick={()=>setTab('insp')}>Inspector</div>
        <div className={'tab ' + (tab==='config'?'active':'')} onClick={()=>setTab('config')}>Config</div>
        <div className={'tab ' + (tab==='runs'?'active':'')} onClick={()=>setTab('runs')}>Runs<span className="count">{allRuns.length}</span></div>
      </div>

      <div className="insp-body scrollnice">
        {tab === 'insp' && <InspectorLiveIO node={node} runtime={runtime} currentRun={currentRun}/>}
        {tab === 'config' && <InspectorConfig node={node} onUpdate={onUpdate}/>}
        {tab === 'runs' && <InspectorRuns node={node} runs={allRuns} currentId={currentRun?.id} onPick={onSelectRun}/>}
      </div>
    </div>
  );
}

function fmt(v) {
  if (v == null) return '—';
  if (typeof v === 'string') return v;
  try { return JSON.stringify(v, null, 2); } catch { return String(v); }
}

function InspectorLiveIO({ node, runtime, currentRun }) {
  const last = node.last || {};
  const snap = currentRun?.nodes?.[node.id] || {};
  const status = runtime?.status || snap.status || last.status || 'idle';
  const live = runtime?.streamingOut;
  const tokens = snap.tokens ?? last.tokens;
  const ms = snap.ms ?? last.ms;

  return (
    <div>
      <div style={{display:'flex',alignItems:'center',gap:6,marginBottom:10,fontSize:11}}>
        <span style={{color:'var(--ink-3)',fontFamily:'JetBrains Mono, monospace'}}>{currentRun?.id || '—'}</span>
        <span style={{color:'var(--ink-4)'}}>·</span>
        <span style={{color:'var(--ink-3)'}}>{currentRun?.whenAbs} · {currentRun?.when}</span>
      </div>
      <div className="stat-grid">
        <div className="stat"><div className="l">Tokens</div><div className="v">{tokens ? tokens.toLocaleString() : '—'}</div></div>
        <div className="stat"><div className="l">Latency</div><div className="v">{ms ? (ms >= 1000 ? `${(ms/1000).toFixed(1)}s` : `${ms}ms`) : '—'}</div></div>
        <div className="stat"><div className="l">Cost</div><div className="v">{tokens ? `$${(tokens * 0.00001).toFixed(3)}` : '—'}</div></div>
      </div>

      {node.inputs.length > 0 && (
        <>
          <div className="io-meta"><b>Input</b><span>{currentRun?.whenAbs || '—'}</span></div>
          {node.inputs.map(p => (
            <div key={p.id} style={{marginBottom:8}}>
              <div style={{display:'flex',alignItems:'center',gap:6,marginBottom:4,fontSize:11,color:'var(--ink-3)'}}>
                <span style={{color:'var(--ink-2)',fontWeight:500}}>{p.label}</span>
                <span style={{fontFamily:'JetBrains Mono, monospace', color:'var(--ink-3)'}}>· {p.type}</span>
              </div>
              <div className="io-card in">{fmt(last.input?.[p.id])}</div>
            </div>
          ))}
        </>
      )}

      <div className="io-meta"><b>Output</b><span>{status === 'run' ? 'streaming…' : (status === 'ok' ? 'completed' : status)}</span></div>
      {node.outputs.map(p => (
        <div key={p.id} style={{marginBottom:8}}>
          <div style={{display:'flex',alignItems:'center',gap:6,marginBottom:4,fontSize:11,color:'var(--ink-3)'}}>
            <span style={{color:'var(--ink-2)',fontWeight:500}}>{p.label}</span>
            <span style={{fontFamily:'JetBrains Mono, monospace', color:'var(--ink-3)'}}>· {p.type}</span>
          </div>
          <div className="io-card out">
            {status === 'run' && live?.[p.id] ? (
              <span>{live[p.id]}<span style={{display:'inline-block',width:7,height:13,background:'var(--accent)',marginLeft:2,verticalAlign:'middle',animation:'pulse 1s infinite'}}/></span>
            ) : (status === 'idle' ? '—' : fmt(last.output?.[p.id]))}
          </div>
        </div>
      ))}
    </div>
  );
}

function InspectorConfig({ node, onUpdate }) {
  const cfg = node.config || {};
  if (node.kind === 'llm') {
    return (
      <div>
        <div className="field-row">
          <label>Model</label>
          <select className="select" defaultValue={cfg.model}>
            <option>{cfg.model}</option>
            <option>gpt-5</option><option>claude-sonnet-4.5</option><option>claude-opus-4.5</option>
          </select>
        </div>
        <div className="row-2">
          <div className="field-row">
            <label>Temperature</label>
            <input className="input" defaultValue={cfg.temperature}/>
          </div>
          <div className="field-row">
            <label>Max tokens</label>
            <input className="input" defaultValue={cfg.max_tokens}/>
          </div>
        </div>
        <div className="field-row">
          <label>System prompt</label>
          <textarea className="textarea" rows="6" defaultValue={cfg.system}/>
        </div>
        <div className="field-row">
          <label>Tools</label>
          <div className="seg">
            <button className="on">None</button>
            <button>Web</button>
            <button>Code</button>
            <button>Custom</button>
          </div>
        </div>
      </div>
    );
  }
  if (node.kind === 'trigger') {
    return (
      <div>
        <div className="field-row"><label>Method</label>
          <div className="seg">
            <button>GET</button><button className="on">POST</button><button>PUT</button><button>ANY</button>
          </div>
        </div>
        <div className="field-row"><label>Path</label><input className="input" defaultValue={cfg.path}/></div>
        <div className="field-row"><label>Auth</label>
          <select className="select" defaultValue={cfg.auth}><option>bearer</option><option>none</option><option>signature</option></select>
        </div>
      </div>
    );
  }
  return (
    <div>
      <div className="field-row"><label>Operation</label>
        <select className="select" defaultValue={cfg.operation}><option>map</option><option>fan_out</option><option>filter</option><option>merge</option></select>
      </div>
      {cfg.concurrency != null && (
        <div className="field-row"><label>Concurrency</label><input className="input" defaultValue={cfg.concurrency}/></div>
      )}
      {cfg.timeout_ms != null && (
        <div className="field-row"><label>Timeout (ms)</label><input className="input" defaultValue={cfg.timeout_ms}/></div>
      )}
      {cfg.provider && (
        <div className="field-row"><label>Provider</label>
          <select className="select" defaultValue={cfg.provider}><option>brave</option><option>tavily</option><option>serper</option></select>
        </div>
      )}
    </div>
  );
}

function InspectorRuns({ node, runs, currentId, onPick }) {
  return (
    <div>
      {runs.map(r => {
        const snap = r.nodes?.[node.id] || {};
        return (
          <div
            key={r.id}
            onClick={() => onPick(r.id)}
            style={{display:'flex',alignItems:'center',gap:10,padding:'10px 4px',borderBottom:'1px solid var(--line)',cursor:'pointer',
              background: r.id === currentId ? 'var(--accent-soft)' : 'transparent',
              borderRadius: r.id === currentId ? 6 : 0,
              marginLeft: r.id === currentId ? -4 : 0, marginRight: r.id === currentId ? -4 : 0,
              paddingLeft: r.id === currentId ? 8 : 4, paddingRight: r.id === currentId ? 8 : 4,
            }}>
            <span className={'pill ' + (snap.status || 'idle')}>
              <span className="dot" style={(snap.status||'idle')==='idle'?{background:'var(--st-idle)'}:undefined}/>{snap.status || 'idle'}
            </span>
            <div style={{flex:1, minWidth:0}}>
              <div style={{fontFamily:'JetBrains Mono, monospace', fontSize:11.5, color:'var(--ink)'}}>{r.id}</div>
              <div style={{fontSize:11, color:'var(--ink-3)'}}>{r.when} · {r.whenAbs}</div>
            </div>
            <div style={{textAlign:'right', fontFamily:'JetBrains Mono, monospace', fontSize:11, color:'var(--ink-2)'}}>
              <div>{snap.ms ? (snap.ms >= 1000 ? `${(snap.ms/1000).toFixed(1)}s` : `${snap.ms}ms`) : '—'}</div>
              <div style={{color:'var(--ink-3)'}}>{snap.tokens ? `${snap.tokens.toLocaleString()} tok` : '—'}</div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

Object.assign(window, { Inspector });
