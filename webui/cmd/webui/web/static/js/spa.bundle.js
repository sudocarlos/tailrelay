(()=>{(()=>{let n={relays:[],proxies:[],showRelays:!0,showProxies:!0,tailnetFQDN:"",logs:[],logLevel:"INFO",logStream:null},l={items:document.getElementById("items"),lastUpdated:document.getElementById("last-updated"),itemCount:document.getElementById("item-count"),alertContainer:document.getElementById("alert-container"),logOutput:document.getElementById("log-output"),logLevel:document.getElementById("log-level"),logLevelSelect:document.getElementById("log-level-select"),refresh:document.getElementById("refresh"),clearLogs:document.getElementById("clear-logs"),filterRelay:document.getElementById("filter-relay"),filterProxy:document.getElementById("filter-proxy")},u=[],r=async(e,a={})=>{let t=await fetch(e,{credentials:"same-origin",headers:{"Content-Type":"application/json",...a.headers||{}},...a});if(!t.ok){let s=await t.text();throw new Error(s||`Request failed: ${t.status}`)}return t.json()},x=()=>{let e=new Date;l.lastUpdated.textContent=e.toLocaleTimeString()},d=(e,a)=>{let t=document.createElement("div");t.className=`alert alert-${e} alert-dismissible fade show`,t.setAttribute("role","alert"),t.innerHTML=`
      <div>${a}</div>
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `,l.alertContainer.appendChild(t),setTimeout(()=>{t.classList.remove("show"),t.addEventListener("transitionend",()=>t.remove())},6e3)},w=e=>`tcp://${n.tailnetFQDN||"unknown"}:${e.listen_port} \u2192 ${e.target_host}:${e.target_port}`,L=e=>{let a=e.port?`:${e.port}`:"",t=`https://${e.hostname}${a}`;return`<a class="proxy-link" href="${t}" target="_blank" rel="noopener">${t}</a>`},p=e=>{l.items.innerHTML=`
      <div class="col-12">
        <div class="card shadow-sm">
          <div class="card-body text-center text-muted">
            ${e}
          </div>
        </div>
      </div>
    `},y=()=>{S();let a=[...n.relays.map(t=>({type:"relay",relay:t.relay,running:t.running})),...n.proxies.map(t=>({type:"proxy",proxy:t}))].filter(t=>t.type==="relay"?n.showRelays:n.showProxies);if(l.itemCount.textContent=`${a.length} item${a.length===1?"":"s"}`,!a.length){!n.showRelays&&!n.showProxies?p("Enable TCP relays or HTTPS proxies to view items."):n.showRelays&&!n.showProxies?p("No TCP relays configured."):!n.showRelays&&n.showProxies?p("No HTTPS proxies configured."):p("No relays or proxies configured.");return}l.items.innerHTML=a.map(t=>{var h;if(t.type==="relay"){let c=t.relay,g=t.running,D=g?"text-bg-success":"text-bg-secondary",O=c.enabled?"text-bg-primary":"text-bg-warning";return`
            <div class="col-12">
              <div class="card shadow-sm h-100">
                <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                  <div class="flex-grow-1">
                    <div class="d-flex align-items-center gap-2 flex-wrap">
                      <span class="badge text-bg-info" data-bs-toggle="tooltip" title="served by socat">TCP Relay</span>
                      <span class="fw-semibold">${w(c)}</span>
                    </div>
                    <div class="small text-muted mt-1">ID: ${c.id}</div>
                  </div>
                  <div class="d-flex align-items-center gap-2">
                    <span class="badge ${D}">${g?"Running":"Stopped"}</span>
                    <span class="badge ${O}">${c.enabled?"Enabled":"Disabled"}</span>
                    <button class="btn btn-outline-secondary btn-sm action-btn" data-type="relay" data-id="${c.id}" data-running="${g}">
                      <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${g?"bi-pause-fill":"bi-play-fill"}"></use></svg>
                      ${g?"Pause":"Start"}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          `}let s=t.proxy,o=s.enabled?"text-bg-success":"text-bg-secondary",i=(h=s.running)!=null?h:s.Running,b=i?"text-bg-success":"text-bg-secondary",B=i?"Caddy Running":"Caddy Down";return`
          <div class="col-12">
            <div class="card shadow-sm h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span class="badge text-bg-primary" data-bs-toggle="tooltip" title="served by caddy">HTTPS Proxy</span>
                    <span class="fw-semibold">${L(s)} \u2192 ${s.target}</span>
                  </div>
                  <div class="small text-muted mt-1">ID: ${s.id}</div>
                </div>
                <div class="d-flex align-items-center gap-2">
                  <span class="badge ${b}">${B}</span>
                  <span class="badge ${o}">${s.enabled?"Enabled":"Disabled"}</span>
                  <button class="btn btn-outline-secondary btn-sm action-btn" data-type="proxy" data-id="${s.id}" data-enabled="${s.enabled}">
                    <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${s.enabled?"bi-pause-fill":"bi-play-fill"}"></use></svg>
                    ${s.enabled?"Pause":"Start"}
                  </button>
                </div>
              </div>
            </div>
          </div>
        `}).join(""),$()},$=()=>{document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach(e=>{u.push(new bootstrap.Tooltip(e))})},S=()=>{for(;u.length;)u.pop().dispose()},m=async()=>{try{let[e,a,t]=await Promise.all([r("/api/socat/relays"),r("/api/caddy/proxies"),r("/api/tailscale/status")]);n.relays=e.map(s=>{var o;return{relay:s.Relay||s.relay,running:(o=s.Running)!=null?o:s.running}}),n.proxies=a.map(s=>{var o;return{...s,running:(o=s.running)!=null?o:s.Running}}),n.tailnetFQDN=t.MagicDNSName||t.magicDNSName||"",y(),x()}catch(e){d("danger",e.message)}},E=async(e,a)=>{let t=a?`/api/socat/stop?id=${encodeURIComponent(e)}`:`/api/socat/start?id=${encodeURIComponent(e)}`;await r(t,{method:"POST"})},T=async(e,a)=>{await r("/api/caddy/toggle",{method:"POST",body:JSON.stringify({id:e,enabled:!a})})},P=async e=>{let a=e.target.closest(".action-btn");if(!a)return;a.disabled=!0;let t=a.dataset.type;try{if(t==="relay"){let s=a.dataset.running==="true";await E(a.dataset.id,s)}else{let s=a.dataset.enabled==="true";await T(a.dataset.id,s)}await m()}catch(s){d("danger",s.message)}finally{a.disabled=!1}},v=e=>{if(!e||!e.message)return;let t=(e.timestamp?new Date(e.timestamp):new Date).toLocaleTimeString(),s=e.source?` [${e.source}]`:"",o=`${t} [${e.level}]${s} ${e.message}`,i=l.logOutput,b=i.scrollTop+i.clientHeight>=i.scrollHeight-8;i.textContent+=`${o}
`,b&&(i.scrollTop=i.scrollHeight)},C=async()=>{try{let e=await r("/api/logs");n.logs=e.logs||[],n.logLevel=e.level||"INFO",l.logLevel.textContent=n.logLevel,l.logLevelSelect&&(l.logLevelSelect.value=n.logLevel),l.logOutput.textContent="",n.logs.forEach(v)}catch(e){d("warning",e.message)}},R=async e=>{try{let a=await r("/api/logs/level",{method:"POST",body:JSON.stringify({level:e})});n.logLevel=a.level||e,l.logLevel.textContent=n.logLevel,l.logLevelSelect&&(l.logLevelSelect.value=n.logLevel)}catch(a){d("warning",a.message)}},I=()=>{n.logStream&&n.logStream.close();let e=new EventSource("/api/logs/stream");e.onmessage=a=>{try{let t=JSON.parse(a.data);if(t.connected)return;v(t)}catch{}},e.onerror=()=>{d("warning","Log stream disconnected. Retrying...")},n.logStream=e},N=()=>{l.items.addEventListener("click",P),l.filterRelay.addEventListener("change",()=>{n.showRelays=l.filterRelay.checked,y()}),l.filterProxy.addEventListener("change",()=>{n.showProxies=l.filterProxy.checked,y()}),l.refresh.addEventListener("click",m),l.clearLogs.addEventListener("click",()=>{l.logOutput.textContent=""}),l.logLevelSelect&&l.logLevelSelect.addEventListener("change",e=>{R(e.target.value)})},f=async()=>{N(),await m(),await C(),I(),setInterval(m,15e3)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",f):f()})();})();
