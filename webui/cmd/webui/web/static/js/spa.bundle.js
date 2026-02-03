(()=>{(()=>{let o={relays:[],proxies:[],showRelays:!0,showProxies:!0,tailnetFQDN:"",logs:[],logLevel:"INFO",logStream:null},n={items:document.getElementById("items"),lastUpdated:document.getElementById("last-updated"),itemCount:document.getElementById("item-count"),alertContainer:document.getElementById("alert-container"),logOutput:document.getElementById("log-output"),logLevel:document.getElementById("log-level"),logLevelSelect:document.getElementById("log-level-select"),refresh:document.getElementById("refresh"),clearLogs:document.getElementById("clear-logs"),filterRelay:document.getElementById("filter-relay"),filterProxy:document.getElementById("filter-proxy"),themeToggle:document.getElementById("theme-toggle")},h=[],S=()=>{let e=localStorage.getItem("theme");return e||(window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light")},b=e=>{document.documentElement.setAttribute("data-bs-theme",e),localStorage.setItem("theme",e),T(e)},T=e=>{if(!n.themeToggle)return;let t=e==="dark"?"bi-moon-stars-fill":"bi-sun-fill";n.themeToggle.querySelector("use").setAttribute("href",`/static/vendor/bootstrap-icons/bootstrap-icons.svg#${t}`)},E=()=>{let t=(document.documentElement.getAttribute("data-bs-theme")||"light")==="dark"?"light":"dark";b(t)},i=async(e,t={})=>{let a=await fetch(e,{credentials:"same-origin",headers:{"Content-Type":"application/json",...t.headers||{}},...t});if(!a.ok){let s=await a.text();throw new Error(s||`Request failed: ${a.status}`)}return a.json()},k=()=>{let e=new Date;n.lastUpdated.textContent=e.toLocaleTimeString()},c=(e,t)=>{let a=document.createElement("div");a.className=`alert alert-${e} alert-dismissible fade show`,a.setAttribute("role","alert"),a.innerHTML=`
      <div>${t}</div>
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `,n.alertContainer.appendChild(a),setTimeout(()=>{a.classList.remove("show"),a.addEventListener("transitionend",()=>a.remove())},6e3)},P=e=>`tcp://${o.tailnetFQDN||"unknown"}:${e.listen_port} \u2192 ${e.target_host}:${e.target_port}`,C=e=>{let t=e.port?`:${e.port}`:"",a=`https://${e.hostname}${t}`;return`<a class="proxy-link" href="${a}" target="_blank" rel="noopener">${a}</a>`},y=e=>{n.items.innerHTML=`
      <div class="col-12">
        <div class="card">
          <div class="card-body text-center text-muted">
            ${e}
          </div>
        </div>
      </div>
    `},f=()=>{R();let t=[...o.relays.map(a=>({type:"relay",relay:a.relay,running:a.running})),...o.proxies.map(a=>({type:"proxy",proxy:a}))].filter(a=>a.type==="relay"?o.showRelays:o.showProxies);if(n.itemCount.textContent=`${t.length} item${t.length===1?"":"s"}`,!t.length){!o.showRelays&&!o.showProxies?y("Enable TCP relays or HTTPS proxies to view items."):o.showRelays&&!o.showProxies?y("No TCP relays configured."):!o.showRelays&&o.showProxies?y("No HTTPS proxies configured."):y("No relays or proxies configured.");return}n.items.innerHTML=t.map(a=>{var w,L,$;if(a.type==="relay"){let u=a.relay,p=a.running,J=p?"text-bg-success":"text-bg-secondary",U=(w=u.autostart)!=null?w:!1;return`
            <div class="col-12">
              <div class="card h-100">
                <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                  <div class="flex-grow-1">
                    <div class="d-flex align-items-center gap-2 flex-wrap">
                      <span class="badge text-bg-info" data-bs-toggle="tooltip" title="served by socat">TCP Relay</span>
                      <span class="fw-semibold">${P(u)}</span>
                    </div>
                    <div class="small text-muted mt-1">ID: ${u.id}</div>
                  </div>
                  <div class="d-flex align-items-center gap-2">
                    <span class="badge ${J}">${p?"Running":"Stopped"}</span>
                    <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                      <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                             ${U?"checked":""} 
                             data-type="relay" data-id="${u.id}">
                      <label class="form-check-label small text-muted">Autostart</label>
                    </div>
                    <button class="btn btn-outline-secondary btn-sm action-btn" data-type="relay" data-id="${u.id}" data-running="${p}">
                      <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${p?"bi-pause-fill":"bi-play-fill"}"></use></svg>
                      ${p?"Pause":"Start"}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          `}let s=a.proxy,l=(L=s.running)!=null?L:s.Running,r=l?"text-bg-success":"text-bg-secondary",d=l?"Caddy Running":"Caddy Down",m=($=s.autostart)!=null?$:!1;return`
          <div class="col-12">
            <div class="card h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span class="badge text-bg-primary" data-bs-toggle="tooltip" title="served by caddy">HTTPS Proxy</span>
                    <span class="fw-semibold">${C(s)} \u2192 ${s.target}</span>
                  </div>
                  <div class="small text-muted mt-1">ID: ${s.id}</div>
                </div>
                <div class="d-flex align-items-center gap-2">
                  <span class="badge ${r}">${d}</span>
                  <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                    <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                           ${m?"checked":""} 
                           data-type="proxy" data-id="${s.id}">
                    <label class="form-check-label small text-muted">Autostart</label>
                  </div>
                  <button class="btn btn-outline-secondary btn-sm action-btn" data-type="proxy" data-id="${s.id}" data-enabled="${s.enabled}">
                    <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${s.enabled?"bi-pause-fill":"bi-play-fill"}"></use></svg>
                    ${s.enabled?"Pause":"Start"}
                  </button>
                </div>
              </div>
            </div>
          </div>
        `}).join(""),I()},I=()=>{document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach(e=>{h.push(new bootstrap.Tooltip(e))})},R=()=>{for(;h.length;)h.pop().dispose()},g=async()=>{try{let[e,t,a]=await Promise.all([i("/api/socat/relays"),i("/api/caddy/proxies"),i("/api/tailscale/status")]);o.relays=e.map(s=>{var l;return{relay:s.Relay||s.relay,running:(l=s.Running)!=null?l:s.running}}),o.proxies=t.map(s=>{var l;return{...s,running:(l=s.running)!=null?l:s.Running}}),o.tailnetFQDN=a.MagicDNSName||a.magicDNSName||"",f(),k()}catch(e){c("danger",e.message)}},N=async(e,t)=>{let a=t?`/api/socat/stop?id=${encodeURIComponent(e)}`:`/api/socat/start?id=${encodeURIComponent(e)}`;await i(a,{method:"POST"})},O=async(e,t)=>{await i("/api/caddy/toggle",{method:"POST",body:JSON.stringify({id:e,enabled:!t})})},B=async(e,t,a)=>{var d;let s=e==="relay"?"/api/socat/update":"/api/caddy/update",l=e==="relay"?(d=o.relays.find(m=>m.relay.id===t))==null?void 0:d.relay:o.proxies.find(m=>m.id===t);if(!l)throw new Error(`${e} not found`);let r={...l,autostart:a};await i(s,{method:"POST",body:JSON.stringify(r)})},D=async e=>{let t=e.target.closest(".action-btn");if(!t)return;t.disabled=!0;let a=t.dataset.type;try{if(a==="relay"){let s=t.dataset.running==="true";await N(t.dataset.id,s)}else{let s=t.dataset.enabled==="true";await O(t.dataset.id,s)}await g()}catch(s){c("danger",s.message)}finally{t.disabled=!1}},A=async e=>{let t=e.target;if(!t.classList.contains("autostart-toggle"))return;let{type:a,id:s}=t.dataset,l=t.checked;t.disabled=!0;try{await B(a,s,l),await g()}catch(r){c("danger",r.message),t.checked=!l}finally{t.disabled=!1}},v=e=>{if(!e||!e.message)return;let a=(e.timestamp?new Date(e.timestamp):new Date).toLocaleTimeString(),s=e.source?` [${e.source}]`:"",l=`${a} [${e.level}]${s} ${e.message}`,r=n.logOutput,d=r.scrollTop+r.clientHeight>=r.scrollHeight-8;r.textContent+=`${l}
`,d&&(r.scrollTop=r.scrollHeight)},H=async()=>{try{let e=await i("/api/logs");o.logs=e.logs||[],o.logLevel=e.level||"INFO",n.logLevel.textContent=o.logLevel,n.logLevelSelect&&(n.logLevelSelect.value=o.logLevel),n.logOutput.textContent="",o.logs.forEach(v)}catch(e){c("warning",e.message)}},M=async e=>{try{let t=await i("/api/logs/level",{method:"POST",body:JSON.stringify({level:e})});o.logLevel=t.level||e,n.logLevel.textContent=o.logLevel,n.logLevelSelect&&(n.logLevelSelect.value=o.logLevel)}catch(t){c("warning",t.message)}},q=()=>{o.logStream&&o.logStream.close();let e=new EventSource("/api/logs/stream");e.onmessage=t=>{try{let a=JSON.parse(t.data);if(a.connected)return;v(a)}catch{}},e.onerror=()=>{c("warning","Log stream disconnected. Retrying...")},o.logStream=e},F=()=>{n.items.addEventListener("click",D),n.items.addEventListener("change",A),n.filterRelay.addEventListener("change",()=>{o.showRelays=n.filterRelay.checked,f()}),n.filterProxy.addEventListener("change",()=>{o.showProxies=n.filterProxy.checked,f()}),n.themeToggle&&n.themeToggle.addEventListener("click",E),n.refresh.addEventListener("click",g),n.clearLogs.addEventListener("click",()=>{n.logOutput.textContent=""}),n.logLevelSelect&&n.logLevelSelect.addEventListener("change",e=>{M(e.target.value)})},x=async()=>{b(S()),F(),await g(),await H(),q(),setInterval(g,15e3)};document.readyState==="loading"?document.addEventListener("DOMContentLoaded",x):x()})();})();
