// Node — visual card with header, body, ports.
// Drag to move; click ports/header to select. Shows status, tokens, last preview.

function StatusPill({ status, tokens, ms, onClick, runId }) {
  const cls = ['pill'];
  if (status === 'run') cls.push('run');
  else if (status === 'ok') cls.push('ok');
  else if (status === 'err') cls.push('err');
  else if (status === 'warn') cls.push('warn');
  else cls.push('idle');
  if (onClick) cls.push('clickable');

  const content = (() => {
    if (status === 'run') return (<>running</>);
    if (status === 'err') return (<>error</>);
    if (status === 'warn') return (<>warn</>);
    if (status === 'ok') {
      const parts = [];
      if (tokens) parts.push(`${tokens.toLocaleString()} tok`);
      if (ms) parts.push(ms >= 1000 ? `${(ms/1000).toFixed(1)}s` : `${ms}ms`);
      return <>{parts.join(' · ') || 'ok'}</>;
    }
    return <>idle</>;
  })();

  return (
    <span className={cls.join(' ')} onClick={onClick ? (e) => { e.stopPropagation(); onClick(e); } : undefined} title={runId ? `Run ${runId}` : undefined}>
      <span className="dot" style={status==='idle'?{background:'var(--st-idle)'}:undefined}/>
      {content}
      {onClick && (
        <svg className="chev" width="9" height="9" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M2.5 4 L5 6.5 L7.5 4"/></svg>
      )}
    </span>
  );
}

function NodePorts({ ports, side }) {
  return (
    <div className={`ports ${side}`}>
      {ports.map(p => (
        <div key={p.id} className={`port ${side} t-${p.type}`} data-port-id={p.id}>
          <span className="dot"/>
          <span className="label">{p.label}</span>
          <span className="type" style={{marginLeft: side==='in' ? 'auto' : 0, marginRight: side==='out' ? 'auto' : 0, order: side==='out' ? 0 : undefined}}>{p.type}</span>
        </div>
      ))}
    </div>
  );
}

function WorkflowNode({ node, selected, onSelect, onDragStart, runtime, currentRun, allRuns, onSelectRun }) {
  const status = runtime?.status || (currentRun?.nodes?.[node.id]?.status) || 'idle';
  const visual = KIND_VISUAL[node.kind] || KIND_VISUAL.llm;
  const cls = ['node', selected ? 'selected' : '', status === 'run' ? 'running' : '', status === 'ok' ? 'ok' : '', status === 'err' ? 'err' : ''].filter(Boolean).join(' ');
  const snap = currentRun?.nodes?.[node.id];
  const tokens = runtime ? undefined : snap?.tokens;
  const ms = runtime ? undefined : snap?.ms;
  const [menuOpen, setMenuOpen] = React.useState(false);

  const previewText = (() => {
    if (status === 'run') return '⋯ streaming';
    const out = node.last?.output;
    if (!out) return null;
    const k = Object.keys(out)[0];
    const v = out[k];
    if (typeof v === 'string') return v;
    if (Array.isArray(v)) return `[${v.length}] ${v[0] || ''}`;
    return String(v);
  })();

  return (
    <div
      className={cls}
      style={{ transform: `translate(${node.x}px, ${node.y}px)` }}
      onMouseDown={(e) => onDragStart(e, node.id)}
      onClick={(e) => { e.stopPropagation(); onSelect(node.id); }}
    >
      <div className="node-head">
        <div className="node-icon" style={{background: visual.bg, color: visual.fg}}>
          <KindIcon kind={node.kind} s={13}/>
        </div>
        <div style={{flex:1, minWidth:0}}>
          <div className="node-name">{node.name}</div>
          <div className="node-sub">{node.sub}</div>
        </div>
        <span className="runmenu-anchor" onMouseDown={(e)=>e.stopPropagation()}>
          <StatusPill
            status={status}
            tokens={tokens}
            ms={ms}
            runId={currentRun?.id}
            onClick={status !== 'run' ? () => setMenuOpen(o => !o) : undefined}
          />
          {menuOpen && (
            <RunMenu
              runs={allRuns}
              currentId={currentRun?.id}
              onPick={(id) => { setMenuOpen(false); onSelectRun(id); }}
              onClose={() => setMenuOpen(false)}
              nodeId={node.id}
              align="right"
              top={22}
            />
          )}
        </span>
      </div>

      <div className="node-body">
        {node.body.map((f, i) => (
          <div key={i} className="field">
            <span>{f.k}</span>
            <span className={'v ' + (f.mono ? 'mono' : '')}>{f.v}</span>
          </div>
        ))}
        {previewText != null && (
          <div className="preview fade">{previewText}</div>
        )}
      </div>

      {node.inputs.length > 0 && <NodePorts ports={node.inputs} side="in"/>}
      {node.outputs.length > 0 && <NodePorts ports={node.outputs} side="out"/>}
    </div>
  );
}

function RunMenu({ runs, currentId, onPick, onClose, nodeId, align='left', top=34, left, right }) {
  const ref = React.useRef(null);
  React.useEffect(() => {
    const onDoc = (e) => { if (ref.current && !ref.current.contains(e.target)) onClose(); };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [onClose]);
  const style = { top };
  if (align === 'right') style.right = 0;
  else if (left != null) style.left = left;
  else style.left = 0;
  if (right != null) style.right = right;

  return (
    <div className="runmenu" ref={ref} style={style}>
      <div className="runmenu-head">
        <span>{nodeId ? `Runs · ${nodeId}` : 'Workflow runs'}</span>
        <span style={{color:'var(--ink-3)'}}>{runs.length}</span>
      </div>
      {runs.map(r => {
        const snap = nodeId ? r.nodes?.[nodeId] : null;
        const sCls = (nodeId ? snap?.status : r.status) || 'ok';
        return (
          <div key={r.id} className={'runmenu-item ' + (r.id === currentId ? 'active' : '')} onClick={() => onPick(r.id)}>
            <span className={'stat ' + (sCls === 'err' ? 'err' : (sCls === 'run' ? 'run' : ''))}/>
            <span className="id">{r.id}</span>
            <span style={{flex:1, minWidth:0, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', fontSize:11, color:'inherit', opacity:.85}}>
              {nodeId && snap ? (
                snap.tokens ? `${snap.tokens.toLocaleString()} tok` : (snap.ms ? `${(snap.ms/1000).toFixed(1)}s` : '—')
              ) : (
                `${(r.duration/1000).toFixed(1)}s · ${r.tokens.toLocaleString()} tok`
              )}
            </span>
            <span className="when">{r.when}</span>
          </div>
        );
      })}
      <div className="runmenu-foot">Showing latest {runs.length} · <a>View all runs →</a></div>
    </div>
  );
}

Object.assign(window, { WorkflowNode, RunMenu, StatusPill });
