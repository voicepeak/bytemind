package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opencode-go/internal/fileops"
	"github.com/opencode-go/internal/llm"
	"github.com/opencode-go/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	http.Server
	llm   *llm.Client
	store *store.Store
	mu    sync.Mutex
}

type WSMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Content string `json:"content"`
}

type FileTree struct {
	Name    string     `json:"name"`
	Path    string     `json:"path"`
	IsDir   bool       `json:"isDir"`
	Children []*FileTree `json:"children,omitempty"`
}

func Start(ctx context.Context, addr string) error {
	s := &Server{}
	var err error
	s.store, err = store.New()
	if err != nil {
		return err
	}
	s.llm = llm.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveUI)
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/read", s.handleRead)
	mux.HandleFunc("/api/write", s.handleWrite)
	mux.HandleFunc("/api/run", s.handleRun)
	mux.HandleFunc("/api/status", s.handleStatus)

	s.Addr = addr
	s.Handler = mux

	go func() {
		<-ctx.Done()
		s.Shutdown(context.Background())
	}()

	log.Printf("🌐 Web UI: http://localhost%s", addr)
	return s.ListenAndServe()
}

func (s *Server) serveUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>OpenCode Go</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:ui-monospace,monospace;background:#1e1e2e;color:#cdd6f4;height:100vh;display:flex;flex-direction:column}
#header{background:#313244;padding:10px 20px;display:flex;align-items:center;gap:15px;border-bottom:1px solid #45475a}
#header h1{font-size:16px;color:#89b4fa}
#dir{color:#a6adc8;font-size:13px}
#main{display:flex;flex:1;overflow:hidden}
#sidebar{width:250px;background:#181825;border-right:1px solid #45475a;display:flex;flex-direction:column}
#sidebar-header{padding:10px;background:#313244;font-size:12px;color:#a6adc8;border-bottom:1px solid #45475a}
#file-tree{flex:1;overflow:auto;padding:10px;font-size:13px}
.file-item{padding:4px 8px;cursor:pointer;border-radius:4px}
.file-item:hover{background:#313244}
.file-item.dir{color:#f9e2af}
#chat-container{flex:1;display:flex;flex-direction:column}
#messages{flex:1;overflow:auto;padding:20px}
.msg{margin-bottom:15px;max-width:90%}
.msg.user{text-align:right}
.msg.assistant{text-align:left}
.msg .role{font-size:11px;color:#6c7086;margin-bottom:4px}
.msg .content{background:#313244;padding:12px;border-radius:12px;white-space:pre-wrap;line-height:1.5}
.msg.user .content{background:#89b4fa;color:#1e1e2e}
#input-area{padding:15px 20px;background:#181825;border-top:1px solid #45475a;display:flex;gap:10px}
#input{flex:1;background:#313244;border:1px solid #45475a;border-radius:8px;padding:12px;color:#cdd6f4;font-family:inherit;font-size:14px;resize:none}
#input:focus{outline:none;border-color:#89b4fa}
#send{background:#89b4fa;color:#1e1e2e;border:none;padding:12px 24px;border-radius:8px;cursor:pointer;font-weight:bold}
#send:hover{background:#b4befe}
#send:disabled{opacity:0.5;cursor:not-allowed}
.typing{color:#6c7086;font-style:italic}
.error{color:#f38ba8}
</style>
</head>
<body>
<div id="header">
  <h1>🤖 OpenCode Go</h1>
  <span id="dir">~</span>
</div>
<div id="main">
  <div id="sidebar">
    <div id="sidebar-header">📁 Files</div>
    <div id="file-tree"></div>
  </div>
  <div id="chat-container">
    <div id="messages"></div>
    <div id="input-area">
      <textarea id="input" placeholder="Ask me anything..." rows="1"></textarea>
      <button id="send">Send</button>
    </div>
  </div>
</div>
<script>
const ws=wsconnect();
const messages=document.getElementById('messages');
const input=document.getElementById('input');
const send=document.getElementById('send');
const fileTree=document.getElementById('file-tree');
const dirEl=document.getElementById('dir');

function wsconnect(){
  const w=new WebSocket('ws://'+location.host+'/ws');
  w.onmessage=e=>{
    const d=JSON.parse(e.data);
    if(d.type==='response'){
      appendMsg('assistant',d.content);
      send.disabled=false;
    }else if(d.type==='error'){
      appendMsg('error',d.content);
      send.disabled=false;
    }
  };
  return w;
}

function appendMsg(role,content){
  const div=document.createElement('div');
  div.className='msg '+role;
  div.innerHTML='<div class="role">'+(role==='user'?'You':role==='error'?'Error':'Assistant')+'</div><div class="content">'+escapeHtml(content)+'</div>';
  messages.appendChild(div);
  messages.scrollTop=messages.scrollHeight;
}

function escapeHtml(t){
  return t.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n/g,'<br>');
}

input.addEventListener('keydown',e=>{
  if(e.key==='Enter'&&!e.shiftKey){
    e.preventDefault();
    sendMessage();
  }
});

send.addEventListener('click',sendMessage);

function sendMessage(){
  const msg=input.value.trim();
  if(!msg)return;
  appendMsg('user',msg);
  input.value='';
  send.disabled=true;
  ws.send(JSON.stringify({type:'chat',message:msg}));
}

function loadFiles(){
  fetch('/api/files').then(r=>r.json()).then(files=>{
    fileTree.innerHTML='';
    files.forEach(f=>{
      const div=document.createElement('div');
      div.className='file-item'+(f.isDir?' dir':'');
      div.textContent=(f.isDir?'📁 ':'📄 ')+f.name;
      div.onclick=()=>{
        if(!f.isDir)readFile(f.path);
      };
      fileTree.appendChild(div);
    });
  });
}

function readFile(path){
  fetch('/api/read?path='+encodeURIComponent(path)).then(r=>r.json()).then(d=>{
	appendMsg("assistant", "📄 "+path+"\n-----\n"+d.content+"\n-----");
  });
}

loadFiles();
setInterval(loadFiles,5000);
</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		json.Unmarshal(data, &msg)

		switch msg["type"] {
		case "chat":
			go s.handleChat(conn, msg["message"].(string))
		case "read":
			s.handleFileRead(conn, msg["path"].(string))
		}
	}
}

func (s *Server) handleChat(conn *websocket.Conn, message string) {
	history := s.store.GetHistory()
	var messages []llm.Message

	messages = append(messages, llm.Message{Role: "system", Content: SYSTEM_PROMPT})

	start := 0
	if len(history) > 20 {
		start = len(history) - 20
	}
	for _, m := range history[start:] {
		messages = append(messages, llm.Message{Role: m.Role, Content: m.Content})
	}

	messages = append(messages, llm.Message{Role: "user", Content: message})
	s.store.AddMessage("user", message)

	resp, err := s.llm.Chat(context.Background(), messages)
	if err != nil {
		conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"content": err.Error(),
		})
		return
	}

	s.store.AddMessage("assistant", resp)
	s.store.Save()

	conn.WriteJSON(map[string]interface{}{
		"type":    "response",
		"content": resp,
	})
}

func (s *Server) handleFileRead(conn *websocket.Conn, path string) {
	content, err := fileops.Read(path)
	if err != nil {
		conn.WriteJSON(map[string]interface{}{
			"type":    "error",
			"content": "Cannot read file: " + err.Error(),
		})
		return
	}
	conn.WriteJSON(map[string]interface{}{
		"type":    "file",
		"path":    path,
		"content": content,
	})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = s.store.Dir()
	}
	files, err := fileops.List(dir, false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(files)
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := fileops.Read(path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"content": content})
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := fileops.Write(req.Path, req.Content); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	cmd := r.URL.Query().Get("cmd")
	if cmd == "" {
		http.Error(w, "cmd required", 400)
		return
	}
	parts := strings.Split(cmd, " ")
	c := exec.Command(parts[0], parts[1:]...)
	c.Dir = s.store.Dir()
	out, err := c.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\nError: " + err.Error()
	}
	json.NewEncoder(w).Encode(map[string]string{"output": result})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hasKey":   s.llm.HasKey(),
		"dir":      s.store.Dir(),
		"ready":    true,
		"version":  "0.1.0",
		"timestamp": time.Now().Unix(),
	})
}

const SYSTEM_PROMPT = `You are an expert AI coding assistant. You can:
- Read, write, and edit files
- Run shell commands  
- Search code with grep
- Execute code in various languages

When asked to write or modify code, provide the file path and content clearly.
Always be concise and helpful.`
