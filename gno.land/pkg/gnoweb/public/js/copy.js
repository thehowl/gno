(()=>{class s{DOM;btnClicked=null;btnClickedIcons=[];constructor(){this.DOM={el:document.querySelector("main")},this.DOM.el&&this.init()}init(){this.bindEvents()}bindEvents(){this.DOM.el?.addEventListener("click",this.handleClick.bind(this))}handleClick(e){let t=e.target.closest("[data-copy-btn]");if(!t)return;this.btnClicked=t,this.btnClickedIcons=Array.from(t.querySelectorAll("[data-copy-icon] > use"));let i=t.getAttribute("data-copy-btn");if(!i)return;let o=this.DOM.el?.querySelector(`[data-copy-content="${i}"]`);o&&this.copyToClipboard(o)}sanitizeContent(e){let n=e.innerHTML.replace(/<span class="chroma-ln">.*?<\/span>/g,""),t=document.createElement("div");return t.innerHTML=n,t.textContent?.trim()||""}toggleIcons(){this.btnClickedIcons.forEach(e=>{e.classList.toggle("hidden")})}showFeedback(){this.btnClicked&&(this.toggleIcons(),window.setTimeout(()=>{this.toggleIcons()},1500))}async copyToClipboard(e){let n=this.sanitizeContent(e);try{await navigator.clipboard.writeText(n),this.showFeedback()}catch(t){console.error("Copy error: ",t),this.showFeedback()}}}document.addEventListener("DOMContentLoaded",()=>new s)})();