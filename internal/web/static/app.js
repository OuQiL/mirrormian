// mirror-mian Web 交互层脚本：只做收发与展示，不含业务逻辑。
(() => {
  'use strict';

  const $ = (sel) => document.querySelector(sel);
  const state = {
    sessionId: null,
    mode: 'special',
    answering: false,
    busy: false,
    keyPoints: [],       // 面试方向清单（准备阶段产物）
    currentKp: null,     // 当前知识点
    completedKps: new Set(), // 已完成知识点
    answered: false,     // 是否已至少答一题（决定结束按钮可用性）
    warmupToken: null,   // 综合面试暖场 token
    warmupPhase: null,   // expectation / intro
    warmupExpectation: '',
    user: null,          // 当前登录用户 { id, email, name }
  };

  // --- 工具 ---
  async function api(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data;
  }

  function esc(s) {
    const div = document.createElement('div');
    div.textContent = s || '';
    return div.innerHTML;
  }

  function addMessage(role, html) {
    const row = document.createElement('div');
    row.className = 'msg-row ' + role;
    // AI 消息左侧显示模拟面试官头像（按面试模式选择图标）
    if (role === 'ai') {
      const av = document.createElement('img');
      av.className = 'msg-avatar';
      av.src = state.mode === 'full' ? 'assets/avatar_full.png' : 'assets/avatar_special.png';
      av.alt = 'AI 面试官';
      row.appendChild(av);
    }
    const msg = document.createElement('div');
    msg.className = 'msg ' + role;
    msg.innerHTML = html;
    row.appendChild(msg);
    $('#messages').appendChild(row);
    $('#messages').scrollTop = $('#messages').scrollHeight;
  }

  function toast(text) {
    const t = $('#toast');
    t.textContent = text;
    t.className = 'show';
    setTimeout(() => (t.className = ''), 3000);
  }

  function markdownToHtml(md) {
    return esc(md)
      .replace(/^### (.*)$/gm, '<h3>$1</h3>')
      .replace(/^## (.*)$/gm, '<h2>$1</h2>')
      .replace(/^# (.*)$/gm, '<h1>$1</h1>')
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/^\s*[-*] (.*)$/gm, '<li>$1</li>')
      .replace(/(<li>[\s\S]*?<\/li>)/g, '<ul>$1</ul>')
      .replace(/\n/g, '<br>');
  }

  // --- 等待动画 ---
  function setBusy(on) {
    state.busy = on;
    $('#btn-send').disabled = on || !state.answering;
    $('#btn-quit').disabled = on;
    $('#loading-indicator').style.display = on ? 'flex' : 'none';
    if (on) {
      $('#messages').scrollTop = $('#messages').scrollHeight;
    }
  }

  // --- 方向状态栏（准备阶段产物） ---
  function renderDirectionBar() {
    const bar = $('#direction-bar');
    if (!state.keyPoints.length) {
      bar.style.display = 'none';
      return;
    }
    bar.style.display = 'flex';
    bar.innerHTML = '';
    state.keyPoints.forEach((kp) => {
      const chip = document.createElement('span');
      chip.className = 'kp-chip';
      if (state.completedKps.has(kp)) {
        chip.classList.add('done');
        chip.textContent = '✓ ' + kp;
      } else if (kp === state.currentKp) {
        chip.classList.add('current');
        chip.innerHTML = '<span class="pulse-dot"></span>' + esc(kp);
      } else {
        chip.textContent = kp;
      }
      bar.appendChild(chip);
    });
  }

  function enterKnowledgePoint(kp) {
    if (kp !== state.currentKp) {
      if (state.currentKp) state.completedKps.add(state.currentKp);
      state.currentKp = kp;
      renderDirectionBar();
    }
  }

  // --- 面试流程 ---
  function showQuestion(q) {
    enterKnowledgePoint(q.knowledge_point);
    const label = q.round > 0 ? `追问 ${q.round}` : '题目';
    const kp = q.knowledge_point ? `<span class="kp">${esc(q.knowledge_point)}</span>` : '';
    addMessage('ai', `<div class="q-label">${label} ${kp}</div><div class="q-text">${esc(q.text)}</div>`);
    state.answering = true;
    $('#answer').disabled = false;
    $('#btn-send').disabled = false;
    $('#btn-quit').disabled = false;
    $('#answer').focus();
  }

  // 加载领域列表到主题下拉（主题只能选择已创建领域，不能自由输入）
  async function loadTopicSelect() {
    try {
      const d = await api('/api/domain', {});
      const list = d.domains || [];
      const sel = $('#topic-select');
      if (!list.length) {
        sel.innerHTML = '<option value="">暂无领域，请先到「领域」页创建</option>';
        sel.disabled = true;
        return;
      }
      sel.innerHTML = '<option value="">选择面试领域…</option>' +
        list.map((x) => `<option value="${esc(x.name)}">${esc(x.name)}</option>`).join('');
      sel.disabled = false;
    } catch (e) {
      toast(e.message);
    }
  }

  async function startInterview() {
    if (state.busy) return;
    const mode = document.querySelector('input[name="mode"]:checked').value;
    const body = { mode };
    if (mode === 'special') {
      body.topic = $('#topic-select').value;
      if (!body.topic) return toast('请选择面试领域');
    } else {
      body.jd = $('#jd').value.trim();
      if (!body.jd) return toast('请粘贴 JD 文本');
      // 简历：只能从已上传中选择
      const rid = $('#resume-select').value;
      if (!rid) return toast('请选择简历（可在「简历」页上传或粘贴）');
      body.resume_id = rid;
      body.warmup = true; // 综合面试暖场
    }
    $('#btn-start').disabled = true;
    setBusy(true);
    try {
      const data = await api('/api/interview/start', body);

      // 综合面试暖场：问期望 → 引导自我介绍 → 期间后台准备
      if (data.warmup) {
        state.mode = mode;
        $('#setup').style.display = 'none';
        $('#chat').style.display = 'block';
        $('#review-panel').style.display = 'none';
        $('#messages').innerHTML = '';
        addMessage('system', '开始综合面试（JD 定向）');
        addMessage('ai', `<div class="q-label">面试官</div>${esc(data.question)}`);
        state.warmupToken = data.token;
        state.answering = true; // 回答「期望」阶段
        state.warmupPhase = 'expectation';
        $('#answer').disabled = false;
        $('#btn-send').disabled = false;
        $('#answer').focus();
        return;
      }

      state.sessionId = data.session_id;
      state.mode = mode;
      state.keyPoints = (data.direction && data.direction.key_points) || [];
      state.currentKp = null;
      state.completedKps = new Set();
      state.answered = false;

      $('#setup').style.display = 'none';
      $('#chat').style.display = 'block';
      $('#review-panel').style.display = 'none';
      $('#messages').innerHTML = '';
      renderDirectionBar();

      const topicLabel = mode === 'special' ? data.topic : 'JD 定向';
      addMessage('system', `开始${mode === 'special' ? '专项' : '综合'}面试（${esc(topicLabel)}）`);
      if (state.keyPoints.length) {
        addMessage('system', `已确定 ${state.keyPoints.length} 个面试方向，将从第一个开始：`);
      }
      showQuestion(data.question);
    } catch (e) {
      toast(e.message);
    } finally {
      $('#btn-start').disabled = false;
      setBusy(false);
    }
  }

  async function sendAnswer() {
    if (!state.answering || state.busy) return;
    const text = $('#answer').value.trim();
    if (!text) return toast('回答不能为空');
    state.answering = false;
    $('#answer').disabled = true;
    addMessage('user', esc(text));
    $('#answer').value = '';

    // 暖场阶段 1：用户回答「想要什么样的面试」→ 200ms 后引导自我介绍
    if (state.warmupPhase === 'expectation') {
      state.warmupExpectation = text;
      state.warmupPhase = 'intro';
      setTimeout(() => {
        addMessage('ai', '<div class="q-label">面试官</div>明白了，那就请你先做个自我介绍吧');
        state.answering = true;
        $('#answer').disabled = false;
        $('#btn-send').disabled = false;
        $('#answer').focus();
      }, 200); // 用户要求的 200ms 延迟
      return;
    }

    // 暖场阶段 2：自我介绍 → 取回后台准备好的第一题
    if (state.warmupPhase === 'intro') {
      state.warmupPhase = null;
      setBusy(true);
      try {
        const data = await api('/api/interview/warmup-complete', {
          token: state.warmupToken,
          expectation: state.warmupExpectation || '',
        });
        state.sessionId = data.session_id;
        state.warmupToken = null;
        showQuestion(data.question);
      } catch (e) {
        toast(e.message);
        state.answering = true;
        $('#answer').disabled = false;
        $('#btn-send').disabled = false;
      } finally {
        setBusy(false);
      }
      return;
    }

    if (!state.sessionId) return;
    setBusy(true);
    try {
      const data = await api('/api/interview/answer', {
        session_id: state.sessionId,
        answer: text,
      });
      state.answered = true;
      if (data.next_question) {
        showQuestion(data.next_question);
      } else if (data.done) {
        showReview(data.review_md);
      }
    } catch (e) {
      toast(e.message);
      state.answering = true;
      $('#answer').disabled = false;
    } finally {
      setBusy(false);
    }
  }

  // 提前结束：对已答题目生成复盘
  async function finishInterview() {
    if (state.busy || !state.sessionId) return;
    if (!state.answered) {
      toast('还没有回答任何题目，无法复盘');
      return;
    }
    if (!confirm('确定结束面试并生成复盘吗？')) return;
    setBusy(true);
    try {
      const data = await api('/api/interview/finish', { session_id: state.sessionId });
      showReview(data.review_md);
    } catch (e) {
      toast(e.message);
    } finally {
      setBusy(false);
    }
  }

  function showReview(md) {
    $('#chat').style.display = 'none';
    $('#review-panel').style.display = 'block';
    $('#review-content').innerHTML = markdownToHtml(md);
    $('#review-panel').scrollIntoView({ behavior: 'smooth' });
  }

  function resetToSetup() {
    state.sessionId = null;
    state.keyPoints = [];
    state.currentKp = null;
    state.completedKps = new Set();
    state.answered = false;
    state.answering = false;
    $('#direction-bar').style.display = 'none';
    $('#review-panel').style.display = 'none';
    $('#chat').style.display = 'none';
    $('#setup').style.display = 'block';
    $('#messages').innerHTML = '';
  }

  // --- 历史面试记录 ---
  function formatTime(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d)) return '—';
    const pad = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  function modeLabel(mode) {
    return mode === 'full' ? '综合面试' : '专项面试';
  }
  async function loadHistory() {
    // 回到列表视图
    $('#history-detail').style.display = 'none';
    $('#history-list').style.display = '';
    try {
      const data = await api('/api/history', {});
      const items = data.items || [];
      const box = $('#history-list');
      if (!items.length) {
        box.innerHTML = '<div class="history-empty">暂无历史面试记录，完成一次「AI 面试」后，即可在此回顾记录与复盘。</div>';
        return;
      }
      box.innerHTML = items.map((it) => {
        const statusText = it.status === 'finished' ? '已结束' : '进行中';
        const statusCls = it.status === 'finished' ? 'finished' : 'ongoing';
        const score = it.avg_score != null ? `平均分 ${it.avg_score}` : '暂无评分';
        const topic = it.topic ? ` · ${esc(it.topic)}` : '';
        const resumeBtn = it.status === 'ongoing' ? `<button class="small history-resume-btn" data-id="${esc(it.id)}">继续面试</button>` : '';
        return `<div class="history-item" data-id="${esc(it.id)}">
          <div class="history-item-top">
            <span class="history-item-title">${modeLabel(it.mode)}${topic}</span>
            <span class="history-badge ${statusCls}">${statusText}</span>
          </div>
          <div class="history-item-meta">
            <span>${it.question_count} 题</span>
            <span>${score}</span>
            <span>${formatTime(it.created_at)}</span>
          </div>
          ${resumeBtn}
        </div>`;
      }).join('');
    } catch (e) {
      $('#history-list').innerHTML = `<div class="history-empty">${esc(e.message)}</div>`;
    }
  }

  async function openHistory(id) {
    try {
      const d = await api('/api/history/detail', { id });
      $('#history-detail').style.display = 'block';
      $('#history-list').style.display = 'none';
      const st = d.status === 'finished' ? '已结束' : '进行中';
      // 「继续面试」仅对进行中的会话开放
      const resumeBtn = $('#btn-history-resume');
      if (d.status === 'ongoing') {
        resumeBtn.style.display = '';
        resumeBtn.dataset.id = d.id;
      } else {
        resumeBtn.style.display = 'none';
        resumeBtn.dataset.id = '';
      }
      $('#history-detail-title').innerHTML =
        `<h3>${esc(modeLabel(d.mode))}${d.direction && d.direction.topic ? ' · ' + esc(d.direction.topic) : ''}</h3>
         <p class="history-item-meta"><span>${st}</span><span>${d.questions.length} 题</span><span>${formatTime(d.created_at)}</span></p>`;
      const rv = $('#history-review');
      const qa = $('#history-qa');
      if (d.review) {
        rv.style.display = '';
        rv.innerHTML = markdownToHtml(d.review);
      } else {
        rv.style.display = 'none';
      }
      const ansMap = {};
      (d.answers || []).forEach((a) => { ansMap[a.question_id] = a; });
      const qCount = (d.questions || []).length;
      if (qCount) {
        qa.innerHTML = `<div class="history-subhead">逐题问答（${qCount}）</div>` + (d.questions || []).map((q) => {
          const a = ansMap[q.id];
          let body;
          if (a && a.text) {
            const sc = a.score > 0 ? `<span class="history-qa-score">评分 ${a.score.toFixed(1)}</span>` : '';
            body = `<div class="history-qa-a">${esc(a.text)}${sc}</div>`;
          } else {
            body = `<div class="history-qa-a hint">未作答</div>`;
          }
          return `<div class="history-qa-item">
            <div class="history-qa-q">Q${q.round === 0 ? '' : `（追问${q.round}）`} · ${esc(q.knowledge_point || '')}</div>
            <div class="history-qa-a" style="margin-bottom:8px">${esc(q.text)}</div>
            ${body}
          </div>`;
        }).join('');
      } else {
        qa.innerHTML = '';
      }
      $('#history-detail').scrollIntoView({ behavior: 'smooth' });
    } catch (e) {
      toast(e.message);
    }
  }

  // 续面：恢复「进行中」面试的现场并继续作答
  async function resumeInterview(id) {
    try {
      const d = await api('/api/history/detail', { id });
      if (d.status !== 'ongoing') {
        toast('该面试已结束，无法继续');
        return;
      }
      // 切到 AI 面试页，重建现场
      goto('interview');
      state.sessionId = d.id;
      state.mode = d.mode;
      state.keyPoints = (d.direction && d.direction.key_points) || [];
      state.currentKp = null;
      state.completedKps = new Set();
      state.answered = false;
      state.answering = false;
      state.warmupToken = null;
      state.warmupPhase = null;

      $('#setup').style.display = 'none';
      $('#chat').style.display = 'block';
      $('#review-panel').style.display = 'none';
      $('#messages').innerHTML = '';
      renderDirectionBar();

      const topicLabel = d.mode === 'special' ? ((d.direction && d.direction.topic) || '') : 'JD 定向';
      addMessage('system', `继续${d.mode === 'special' ? '专项' : '综合'}面试（${esc(topicLabel)}）`);
      addMessage('system', `已进行 ${(d.answers || []).length} 个回答，从当前位置继续：`);

      // 重建已答对话
      const ansMap = {};
      (d.answers || []).forEach((a) => { ansMap[a.question_id] = a; });
      const qs = d.questions || [];
      const answeredQs = qs.filter((q) => ansMap[q.id] && ansMap[q.id].text);
      for (const q of answeredQs) {
        const label = q.round > 0 ? `追问 ${q.round}` : '题目';
        const kp = q.knowledge_point ? `<span class="kp">${esc(q.knowledge_point)}</span>` : '';
        addMessage('ai', `<div class="q-label">${label} ${kp}</div><div class="q-text">${esc(q.text)}</div>`);
        addMessage('user', esc(ansMap[q.id].text));
      }
      if (answeredQs.length) state.answered = true;

      // 恢复状态机位置（按已答题目重放 enterKnowledgePoint）
      for (const q of answeredQs) enterKnowledgePoint(q.knowledge_point);

      // 进行中的会话以「最后一题（未答）」为当前题
      const current = qs[qs.length - 1];
      if (current) {
        showQuestion(current);
      } else {
        state.answering = false;
        toast('该会话没有待答题，请重新开始面试');
      }
    } catch (e) {
      toast(e.message);
    }
  }

  // --- 画像 & 复习 ---
  async function loadProfile() {
    try {
      const p = await api('/api/profile', {});
      const html = [];
      const mastery = Object.values(p.mastery || {});
      html.push('<h3>掌握度</h3>');
      if (!mastery.length) html.push('<p class="hint">暂无数据，先跑一场面试</p>');
      for (const m of mastery) {
        html.push(`<div class="bar-row"><span>${esc(m.topic)}</span>
          <div class="bar"><div class="bar-fill" style="width:${Math.min(100, m.score)}%"></div></div>
          <span>${m.score.toFixed(1)}（${m.session_count} 场）</span></div>`);
      }
      html.push('<h3>薄弱点</h3>');
      const wp = p.weak_points || [];
      if (!wp.length) html.push('<p class="hint">暂无薄弱点记录</p>');
      for (const w of wp) {
        const tag = w.improved ? '已改进' : `复习 ${esc(w.sr.next_review)}`;
        html.push(`<div class="wp"><span class="kp">${esc(w.topic)}</span> ${esc(w.point)}
          <span class="meta">×${w.times_seen} · ${tag}</span></div>`);
      }
      html.push('<h3>知识库</h3>');
      try {
        const kb = await api('/api/kb', {});
        const topics = Object.entries(kb.topics || {});
        if (!topics.length) html.push('<p class="hint">暂无知识库内容（CLI 导入：mian kb import &lt;topic&gt; &lt;file&gt;）</p>');
        for (const [topic, n] of topics) {
          html.push(`<div class="wp"><span class="kp">${esc(topic)}</span> ${n} 块 · 出题时自动检索</div>`);
        }
      } catch {
        html.push('<p class="hint">知识库状态不可用</p>');
      }
      $('#profile-content').innerHTML = html.join('');
    } catch (e) {
      toast(e.message);
    }
  }

  async function loadReview() {
    try {
      const d = await api('/api/review', {});
      const due = d.due || [];
      const html = [];
      if (!due.length) html.push('<p class="hint">没有到期的复习项 🎉</p>');
      for (const it of due) {
        html.push(`<div class="wp"><span class="kp">${esc(it.topic)}</span> ${esc(it.point)}
          <span class="meta">×${it.times_seen} · 难度 ${it.ease_factor.toFixed(2)}</span></div>`);
      }
      $('#review-list').innerHTML = html.join('');
    } catch (e) {
      toast(e.message);
    }
  }

  // --- 领域管理 ---
  const domainState = { current: null, files: {}, contents: {}, selectedFile: null };

  async function loadDomains() {
    try {
      const d = await api('/api/domain', {});
      const list = d.domains || [];
      const html = [];
      if (!list.length) html.push('<p class="hint">暂无领域。输入名称创建第一个领域。</p>');
      for (const dom of list) {
        const mastery = dom.stats && dom.stats.mastery > 0 ? `掌握度 ${dom.stats.mastery.toFixed(1)}` : '未训练';
        html.push(`<div class="wp domain-card">
          <div class="domain-info">
            <span class="kp">${esc(dom.name)}</span>
            <span class="meta">${dom.files.length} 文件 · ${dom.chunk_count} 块 · ${dom.stats ? dom.stats.session_count : 0} 场 · ${mastery}</span>
          </div>
          <div class="domain-actions">
            <button class="small" onclick="window.__gotoDomain('${esc(dom.name)}')">管理内容</button>
            <button class="small" onclick="window.__renameDomain('${esc(dom.name)}')">重命名</button>
            <button class="small danger" onclick="window.__deleteDomain('${esc(dom.name)}')">删除</button>
          </div>
        </div>`);
      }
      $('#domain-list').innerHTML = html.join('');
    } catch (e) {
      toast(e.message);
    }
  }

  window.__gotoDomain = async (name) => {
    domainState.current = name;
    try {
      const d = await api('/api/domain/detail', { topic: name });
      domainState.files = d.files || [];
      domainState.contents = d.contents || {};
      $('#domain-detail-title').textContent = `领域：${name}`;
      // 显示详情容器（否则点击后界面无反应）
      $('#domain-detail').style.display = 'block';
      $('#domain-files').innerHTML = domainState.files
        .map((f) => `<button class="file-chip ${f === 'high_freq.md' ? 'hf' : ''}" onclick="window.__openFile('${esc(f)}')">${esc(f)}</button>`)
        .join('');
      // 填充文件下拉框选项
      $('#domain-file-select').innerHTML = domainState.files
        .map((f) => `<option value="${esc(f)}">${esc(f)}</option>`)
        .join('');
      $('#domain-editor').style.display = 'block';
      if (domainState.files.length) window.__openFile(domainState.files[0]);
    } catch (e) {
      toast(e.message);
    }
  };

  window.__openFile = (file) => {
    domainState.selectedFile = file;
    $('#domain-file-select').value = file;
    $('#domain-file-content').value = domainState.contents[file] || '';
  };

  window.__renameDomain = async (oldName) => {
    const newName = prompt(`重命名领域「${oldName}」为：`, oldName);
    if (!newName || newName === oldName) return;
    try {
      await api('/api/domain/rename', { old_name: oldName, new_name: newName });
      toast('已重命名');
      loadDomains();
    } catch (e) {
      toast(e.message);
    }
  };

  window.__deleteDomain = async (name) => {
    if (!confirm(`删除领域「${name}」？将同时清除其知识库与画像数据。`)) return;
    try {
      await api('/api/domain/delete', { topic: name });
      toast('已删除');
      loadDomains();
    } catch (e) {
      toast(e.message);
    }
  };

  async function saveDomainFile() {
    if (!domainState.current || !domainState.selectedFile) return;
    try {
      await api('/api/domain/save-file', {
        topic: domainState.current,
        file: domainState.selectedFile,
        content: $('#domain-file-content').value,
      });
      toast('已保存，请点击「同步向量」使内容生效');
    } catch (e) {
      toast(e.message);
    }
  }

  async function syncDomain() {
    if (!domainState.current) return;
    setBusy(true);
    try {
      const d = await api('/api/domain/sync', { topic: domainState.current });
      toast(`同步完成：${d.chunks} 块`);
      loadDomains();
    } catch (e) {
      toast(e.message);
    } finally {
      setBusy(false);
    }
  }

  // AI 生成核心知识梳理（提示词对齐 TechSpar generateCore）
  async function generateCore() {
    if (!domainState.current) return;
    if (!confirm(`用 AI 为「${domainState.current}」生成核心知识梳理并覆盖 README.md？`)) return;
    setBusy(true);
    try {
      const d = await api('/api/domain/generate-core', { topic: domainState.current });
      domainState.contents['README.md'] = d.content;
      toast('已生成并同步，内容立即参与出题');
      window.__openFile('README.md');
      loadDomains();
    } catch (e) {
      toast(e.message);
    } finally {
      setBusy(false);
    }
  }

  // --- 复习计划 ---
  async function loadPlan() {
    setBusy(true);
    $('#plan-result').innerHTML = '<p class="hint">生成复习计划中…</p>';
    try {
      const res = await fetch('/api/plan', { method: 'POST', body: '{}', headers: { 'Content-Type': 'application/json' } });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      const items = data.items || [];
      if (!items.length) {
        $('#plan-result').innerHTML = `<div class="wp">${esc(data.summary || '暂无行动项')}</div>`;
        return;
      }
      const mark = { high: '🔴', medium: '🟡', low: '🟢' };
      $('#plan-result').innerHTML = `<div class="wp"><strong>${esc(data.summary || '今日计划')}</strong></div>` + items
        .map((it, i) => `<div class="wp">
          <div><strong>${i + 1}. ${mark[it.priority] || '⚪'} ${esc(it.action)}</strong> <span class="kp">${esc(it.topic)}</span>
          ${it.minutes ? `<span class="meta">约 ${it.minutes} 分钟</span>` : ''}</div>
          ${it.suggestion ? `<div class="meta">${esc(it.suggestion)}</div>` : ''}</div>`)
        .join('');
    } catch (e) {
      $('#plan-result').innerHTML = '';
      toast(e.message);
    } finally {
      setBusy(false);
    }
  }

  // --- 资源推荐 ---
  async function loadResources() {
    const topic = $('#resources-topic').value.trim();
    if (!topic) return toast('请输入要推荐的领域/主题');
    setBusy(true);
    $('#resources-result').innerHTML = '<p class="hint">搜索 GitHub 中（首次需下载 MCP Server，可能较慢）…</p>';
    try {
      const res = await fetch('/api/resources?topic=' + encodeURIComponent(topic));
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      const repos = data.repos || [];
      if (!repos.length) {
        $('#resources-result').innerHTML = '<p class="hint">没有找到相关项目</p>';
        return;
      }
      $('#resources-result').innerHTML = '<h3>「' + esc(topic) + '」推荐资源</h3>' + repos
        .map((r) => `<div class="wp"><strong>★${r.stars}</strong> <a href="${esc(r.url)}" target="_blank" rel="noopener">${esc(r.name)}</a>
          <div class="meta">${esc((r.desc || '（无描述）').slice(0, 120))}</div></div>`)
        .join('');
    } catch (e) {
      $('#resources-result').innerHTML = '';
      toast(e.message);
    } finally {
      setBusy(false);
    }
  }

  // --- 简历管理 ---
  async function loadResumes() {
    try {
      const d = await api('/api/resume', {});
      const list = d.resumes || [];
      const html = [];
      if (!list.length) html.push('<p class="hint">暂无简历，上传第一份（PDF/DOCX/TXT）</p>');
      for (const r of list) {
        html.push(`<div class="wp resume-card">
          <div class="domain-info">
            <span class="kp">${esc(r.ext.toUpperCase())}</span>
            <strong>${esc(r.filename)}</strong>
            <span class="meta">${(r.size_bytes / 1024).toFixed(0)} KB · ${r.text_len} 字符 · ${esc(r.created_at)}</span>
          </div>
          <div class="domain-actions">
            <button class="small" onclick="window.__showResume('${esc(r.id)}')">查看解析结构</button>
            <button class="small danger" onclick="window.__deleteResume('${esc(r.id)}')">删除</button>
          </div>
        </div>`);
      }
      $('#resume-list').innerHTML = html.join('');
      // 同步刷新综合面试的简历下拉
      const sel = $('#resume-select');
      const cur = sel.value;
      sel.innerHTML = '<option value="">选择已上传简历（推荐）…</option>' +
        list.map((r) => `<option value="${esc(r.id)}">${esc(r.filename)}</option>`).join('');
      sel.value = cur;
    } catch (e) {
      toast(e.message);
    }
  }

  window.__showResume = async (id) => {
    try {
      const r = await api('/api/resume/' + id, {});
      const win = window.open('', '_blank');
      win.document.write(`<html><head><title>${esc(r.filename)}</title>
        <style>body{font-family:sans-serif;max-width:760px;margin:24px auto;padding:0 20px;line-height:1.7;color:#222}
        h1,h2{border-bottom:1px solid #ddd;padding-bottom:4px}</style></head>
        <body>${markdownToHtml(r.text)}</body></html>`);
      win.document.close();
    } catch (e) {
      toast(e.message);
    }
  };

  window.__deleteResume = async (id) => {
    if (!confirm('删除这份简历？')) return;
    try {
      await api('/api/resume/delete/' + id, {});
      toast('已删除');
      loadResumes();
    } catch (e) {
      toast(e.message);
    }
  };

  async function uploadResume() {
    const input = $('#resume-file');
    if (!input.files || !input.files[0]) return toast('请先选择文件');
    const form = new FormData();
    form.append('file', input.files[0]);
    $('#resume-upload-result').textContent = '解析上传中…';
    $('#resume-upload-result').className = 'test-result';
    try {
      const res = await fetch('/api/resume/upload', { method: 'POST', body: form });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      $('#resume-upload-result').textContent = `✓ 已上传「${data.filename}」`;
      $('#resume-upload-result').className = 'test-result ok';
      input.value = '';
      loadResumes();
    } catch (e) {
      $('#resume-upload-result').textContent = '✗ ' + e.message;
      $('#resume-upload-result').className = 'test-result err';
    }
  }

  // 粘贴文本保存为简历
  async function saveResumeText() {
    const name = $('#resume-text-name').value.trim();
    const text = $('#resume-text-content').value.trim();
    if (!text) return toast('请粘贴简历内容');
    $('#resume-text-result').textContent = '保存中…';
    $('#resume-text-result').className = 'test-result';
    try {
      const res = await fetch('/api/resume/text', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, text }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      $('#resume-text-result').textContent = `✓ 已保存「${data.filename}」`;
      $('#resume-text-result').className = 'test-result ok';
      $('#resume-text-name').value = '';
      $('#resume-text-content').value = '';
      loadResumes();
    } catch (e) {
      $('#resume-text-result').textContent = '✗ ' + e.message;
      $('#resume-text-result').className = 'test-result err';
    }
  }

  // 成长 Chat（技能对话统一入口）
  const growState = { sessionId: null, busy: false, skill: null };

  // 初始化「猜你先说」模块：点击示例语句 → 意图自动匹配并启动技能
  async function loadGrowChips() {
    const items = document.querySelectorAll('.grow-guess-item');
    items.forEach((el) => {
      el.onclick = () => {
        const input = el.getAttribute('data-input') || '';
        window.__startGrowSkill('', input);
      };
    });
  }

  // 启动技能（chips 或意图匹配共用）
  window.__startGrowSkill = async (name, input) => {
    closeGrowHistory();
    growState.sessionId = null;
    growState.skill = null;
    $('#grow-messages').innerHTML = '';
    growShowLoading();
    try {
      const res = await fetch('/api/skill/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, input }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      growState.sessionId = data.session_id;
      growState.skill = data.skill;
      addGrowMsg('ai', growAiReply(data.skill, data.reply));
      if (data.finished) growState.sessionId = null;
    } catch (e) {
      toast(e.message);
    } finally {
      growHideLoading();
    }
  };

  function addGrowMsg(role, text) {
    const row = document.createElement('div');
    row.className = 'msg-row ' + role;
    // AI 消息左侧显示成长 Chat 头像
    if (role === 'ai') {
      const av = document.createElement('img');
      av.className = 'msg-avatar';
      av.src = 'assets/avatar_grow.png';
      av.alt = 'AI 助教';
      row.appendChild(av);
    }
    const msg = document.createElement('div');
    msg.className = 'msg ' + role;
    msg.innerHTML = markdownToHtml(text);
    row.appendChild(msg);
    $('#grow-messages').appendChild(row);
    $('#grow-messages').scrollTop = $('#grow-messages').scrollHeight;
  }

  // 组装 AI 回复：普通对话（未命中技能）不带技能标签前缀
  function growAiReply(skill, reply) {
    return skill && skill !== '普通对话' ? `[${skill}]\n${reply}` : reply;
  }

  // 加载转圈：AI 回复生成中的提示气泡
  let _growLoadingEl = null;
  function growShowLoading() {
    if (_growLoadingEl) return;
    const row = document.createElement('div');
    row.className = 'msg-row ai';
    const av = document.createElement('img');
    av.className = 'msg-avatar';
    av.src = 'assets/avatar_grow.png';
    av.alt = 'AI 助教';
    row.appendChild(av);
    const msg = document.createElement('div');
    msg.className = 'msg ai grow-loading';
    msg.innerHTML = '<span class="spinner"></span><span>么么思考中…</span>';
    row.appendChild(msg);
    $('#grow-messages').appendChild(row);
    $('#grow-messages').scrollTop = $('#grow-messages').scrollHeight;
    _growLoadingEl = row;
  }
  function growHideLoading() {
    if (!_growLoadingEl) return;
    _growLoadingEl.remove();
    _growLoadingEl = null;
  }

  // 发送：无会话 → 意图匹配启动；有会话 → turn 推进
  async function growSend() {
    if (growState.busy) return;
    const text = $('#grow-input').value.trim();
    if (!text) return;
    $('#grow-input').value = '';
    addGrowMsg('user', text);
    growState.busy = true;
    $('#btn-grow-send').disabled = true;
    growShowLoading();
    try {
      let data;
      if (!growState.sessionId) {
        // 意图匹配启动
        const res = await fetch('/api/skill/start', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: '', input: text }),
        });
        data = await res.json();
        if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
        growState.sessionId = data.session_id;
        growState.skill = data.skill;
        addGrowMsg('ai', growAiReply(data.skill, data.reply));
      } else {
        const res = await fetch('/api/skill/turn', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ session_id: growState.sessionId, input: text }),
        });
        data = await res.json();
        if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
        addGrowMsg('ai', data.reply);
      }
      if (data.finished) growState.sessionId = null;
    } catch (e) {
      toast(e.message);
    } finally {
      growHideLoading();
      growState.busy = false;
      $('#btn-grow-send').disabled = false;
    }
  }

  // 新对话
  function growClear() {
    growState.sessionId = null;
    growState.skill = null;
    $('#grow-messages').innerHTML = '';
    closeGrowHistory();
    addGrowMsg('system', '你好，我是么么，你的 AI 学习与面试助手。你可以直接描述需求，例如「考考我 Redis」「安排我的复习计划」；也可以点选下方「猜你先说」快速开始。我会根据你的意图匹配对应技能，未命中技能时也会与你自然对话。');
  }

  // --- 么么Chat 历史记录 ---
  let growHistoryCache = [];

  // 拉取历史会话列表（含完整对话转写）并渲染
  async function loadGrowHistory() {
    try {
      const res = await fetch('/api/skill/history');
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      growHistoryCache = data.sessions || [];
      renderGrowHistory();
    } catch (e) {
      toast(e.message);
    }
  }

  function renderGrowHistory() {
    const list = $('#grow-history-list');
    if (!growHistoryCache.length) {
      list.innerHTML = '<div class="grow-history-empty">暂无历史聊天，去聊两句吧</div>';
      return;
    }
    list.innerHTML = growHistoryCache.map((s) => {
      const first = s.messages && s.messages.length ? s.messages[0] : '';
      const preview = first.length > 42 ? first.slice(0, 42) + '…' : first;
      const skillLabel = s.skill_name && s.skill_name !== '普通对话' ? s.skill_name : '么么Chat';
      const stateText = s.finished ? '已经结束' : '正在进行';
      return `<div class="grow-history-item" onclick="window.__openGrowHistory('${esc(s.id)}')">
        <div class="grow-history-item-top">
          <span class="kp">${esc(skillLabel)}</span>
          <span class="meta">${stateText} · ${esc(s.updated_at)}</span>
        </div>
        <div class="grow-history-item-preview">${esc(preview || '（空会话）')}</div>
      </div>`;
    }).join('');
  }

  // 从历史列表打开某次会话：回放对话；未结束的会话可继续聊
  window.__openGrowHistory = (id) => {
    const sess = growHistoryCache.find((s) => s.id === id);
    if (!sess) return;
    growState.sessionId = null;
    growState.skill = sess.skill_name || null;
    $('#grow-messages').innerHTML = '';
    const msgs = sess.messages || [];
    for (let i = 0; i < msgs.length; i++) {
      addGrowMsg(i % 2 === 0 ? 'user' : 'ai', msgs[i]);
    }
    if (!sess.finished) {
      growState.sessionId = sess.id; // 未结束：可续聊
    }
    closeGrowHistory();
    $('#grow-input').focus();
  };

  function openGrowHistory() {
    loadGrowHistory();
    $('#grow-history').style.display = 'flex';
  }

  function closeGrowHistory() {
    $('#grow-history').style.display = 'none';
  }

  // --- 设置 ---
  const settings = {
    llm: { base_url: '', api_key: '', model: '' },
    embedding: { base_url: '', api_key: '', model: '' },
  };

  async function loadSettings() {
    try {
      const res = await fetch('/api/settings');
      const data = await res.json();
      if (!res.ok) throw new Error((data.error || '') + `HTTP ${res.status}`);
      settings.llm = data.llm || {};
      settings.embedding = data.embedding || {};
      // 回填（key 明文完整回填，原样显示可直接修改）
      $('#set-llm-baseurl').value = settings.llm.base_url || '';
      $('#set-llm-key').value = settings.llm.api_key || '';
      $('#set-llm-model').value = settings.llm.model || '';
      $('#set-emb-baseurl').value = settings.embedding.base_url || '';
      $('#set-emb-key').value = settings.embedding.api_key || '';
      $('#set-emb-model').value = settings.embedding.model || '';
    } catch (e) {
      toast('读取设置失败：' + e.message);
    }
  }

  function collectSettings() {
    return {
      llm: {
        base_url: $('#set-llm-baseurl').value.trim(),
        api_key: $('#set-llm-key').value.trim(),
        model: $('#set-llm-model').value.trim(),
      },
      embedding: {
        base_url: $('#set-emb-baseurl').value.trim(),
        api_key: $('#set-emb-key').value.trim(),
        model: $('#set-emb-model').value.trim(),
      },
    };
  }

  function showTestResult(el, res) {
    const r = $(el);
    if (res.ok) {
      const extra = res.dim ? `（维度 ${res.dim}）` : `（${res.model || ''}）`;
      r.textContent = '✓ 连接成功' + extra;
      r.className = 'test-result ok';
    } else {
      r.textContent = '✗ ' + (res.error || '失败');
      r.className = 'test-result err';
    }
  }

  async function testSettings(group) {
    const payload = collectSettings();
    // 未填 key 时用已配置值（placeholder 语义）
    if (!payload[group].api_key) {
      payload[group].api_key = settings[group].api_key_masked ? '__KEEP__' : '';
    }
    const resultEl = group === 'llm' ? '#llm-test-result' : '#emb-test-result';
    $(resultEl).textContent = '测试中…';
    $(resultEl).className = 'test-result';
    try {
      const res = await fetch('/api/settings/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      showTestResult(resultEl, data[group]);
    } catch (e) {
      $(resultEl).textContent = '✗ ' + e.message;
      $(resultEl).className = 'test-result err';
    }
  }

  async function saveSettings() {
    const payload = collectSettings();
    for (const [k, v] of Object.entries(payload)) {
      if (!v.base_url || !v.model) {
        toast(`${k === 'llm' ? 'LLM' : 'Embedding'} 的 Base URL 与模型不能为空`);
        return;
      }
    }
    $('#settings-save-result').textContent = '保存中…';
    try {
      const res = await fetch('/api/settings/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      $('#settings-save-result').textContent = '✓ ' + (data.message || '已保存');
      $('#settings-save-result').className = 'test-result ok';
      loadSettings();
    } catch (e) {
      $('#settings-save-result').textContent = '✗ ' + e.message;
      $('#settings-save-result').className = 'test-result err';
    }
  }

  // --- 个人账户（登录/注册/登出/会话校验） ---
  function errMsg(e) {
    return (e && e.message) || '操作失败';
  }

  function showAuthOverlay(show) {
    $('#auth-overlay').style.display = show ? 'flex' : 'none';
  }

  function setAuthMessage(text, isErr) {
    const m = $('#auth-msg');
    if (!text) { m.textContent = ''; m.className = 'auth-msg'; return; }
    m.textContent = text;
    m.className = 'auth-msg ' + (isErr ? 'err' : 'ok');
  }

  // 用账户状态渲染标题栏（登录按钮 / 用户信息）
  function renderAccount() {
    const user = state.user;
    $('#btn-login').style.display = user ? 'none' : 'inline-flex';
    $('#user-menu').style.display = user ? 'block' : 'none';
    if (!user) return;
    const name = user.name && user.name.trim() ? user.name.trim() : user.email.split('@')[0];
    $('#user-name').textContent = name;
    $('#dropdown-name').textContent = name;
    $('#dropdown-email').textContent = user.email;
    $('#user-avatar').textContent = name.charAt(0).toUpperCase();
    closeDropdown();
  }

  function closeDropdown() {
    $('#user-dropdown').classList.remove('open');
  }

  async function doLogin(email, password) {
    const res = await fetch('/api/auth/login', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '登录失败');
    return data.user;
  }

  async function doRegister(email, password, name) {
    const res = await fetch('/api/auth/register', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, name: name || '' }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '注册失败');
    return data.user;
  }

  async function doLogout() {
    await fetch('/api/auth/logout', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
  }

  // 校验当前登录态；未登录返回 null（由调用方决定是否拦截）。
  async function checkAuth() {
    const res = await fetch('/api/auth/me');
    if (res.status === 401) return null;
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || '会话校验失败');
    return data.user || null;
  }

  // 登录后一次性加载需要账户数据的依赖（主题下拉 / 简历 / 技能）
  function initAuthedData() {
    loadTopicSelect();
    loadResumes();
    loadGrowChips();
  }

  function switchAuthTab(tab) {
    document.querySelectorAll('.auth-tab').forEach((b) =>
      b.classList.toggle('active', b.dataset.authTab === tab));
    $('#auth-login').style.display = tab === 'login' ? 'block' : 'none';
    $('#auth-register').style.display = tab === 'register' ? 'block' : 'none';
    setAuthMessage('');
  }

  function onAuthed(user) {
    state.user = user;
    renderAccount();
    showAuthOverlay(false);
    initAuthedData();
  }

  // --- 导航（左侧活动栏，VSCode 风格） ---
  const panels = { home: '#panel-home', interview: '#panel-interview', grow: '#panel-grow', profile: '#panel-profile', review: '#panel-review', domain: '#panel-domain', resume: '#panel-resume', history: '#panel-history', settings: '#panel-settings' };
  function goto(name) {
    for (const [k, sel] of Object.entries(panels)) {
      $(sel).style.display = k === name ? 'block' : 'none';
    }
    document.querySelectorAll('.activity-item').forEach((b) => b.classList.toggle('active', b.dataset.panel === name));
    const titles = { home: '首页', interview: 'AI 面试', grow: '么么Chat', profile: '我的画像', review: '复习计划', domain: '领域管理', resume: '简历管理', history: '历史面试', settings: '系统设置' };
    $('#titlebar-context').textContent = titles[name] || '首页';
    if (name === 'profile') loadProfile();
    if (name === 'review') loadReview();
    if (name === 'domain') loadDomains();
    if (name === 'resume') loadResumes();
    if (name === 'history') loadHistory();
    if (name === 'grow') { loadGrowChips(); if (!$('#grow-messages').children.length) growClear(); }
    if (name === 'settings') loadSettings();
  }

  // --- 事件 ---
  document.querySelectorAll('input[name="mode"]').forEach((r) =>
    r.addEventListener('change', () => {
      const full = document.querySelector('input[name="mode"]:checked').value === 'full';
      $('#fields-special').style.display = full ? 'none' : 'block';
      $('#fields-full').style.display = full ? 'block' : 'none';
    }),
  );
  $('#btn-start').addEventListener('click', startInterview);
  $('#btn-send').addEventListener('click', sendAnswer);
  $('#answer').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendAnswer();
    }
  });
  $('#btn-quit').addEventListener('click', finishInterview);
  $('#btn-again').addEventListener('click', resetToSetup);
  // 历史面试：列表项点击进入详情；「继续面试」按钮直达续面
  $('#history-list').addEventListener('click', (e) => {
    const btn = e.target.closest('.history-resume-btn');
    if (btn) {
      e.stopPropagation();
      if (btn.dataset.id) resumeInterview(btn.dataset.id);
      return;
    }
    const item = e.target.closest('.history-item');
    if (item && item.dataset.id) openHistory(item.dataset.id);
  });
  // 历史面试：详情页「继续面试」
  $('#btn-history-resume').addEventListener('click', () => {
    const id = $('#btn-history-resume').dataset.id;
    if (id) resumeInterview(id);
  });
  // 历史面试：返回列表
  $('#btn-history-back').addEventListener('click', loadHistory);
  // 左侧活动栏导航
  document.querySelectorAll('.activity-item').forEach((b) =>
    b.addEventListener('click', () => goto(b.dataset.panel)),
  );
  // 首页卡片 / CTA 快捷入口
  document.querySelectorAll('[data-goto]').forEach((el) =>
    el.addEventListener('click', () => goto(el.dataset.goto)),
  );
  $('#btn-domain-create').addEventListener('click', async () => {
    const name = $('#new-domain-name').value.trim();
    if (!name) return toast('请输入领域名');
    try {
      await api('/api/domain/create', { name });
      $('#new-domain-name').value = '';
      toast(`已创建领域「${name}」，可用「AI 生成核心知识梳理」快速填充内容`);
      loadDomains();
      loadTopicSelect(); // 同步刷新面试页的主题下拉
    } catch (e) {
      toast(e.message);
    }
  });
  $('#domain-file-select').addEventListener('change', (e) => window.__openFile(e.target.value));
  $('#btn-domain-save').addEventListener('click', saveDomainFile);
  $('#btn-domain-sync').addEventListener('click', syncDomain);
  $('#btn-domain-generate').addEventListener('click', generateCore);
  $('#btn-domain-back').addEventListener('click', () => {
    $('#domain-detail').style.display = 'none';
    loadDomains();
  });
  $('#btn-resources').addEventListener('click', loadResources);
  $('#resources-topic').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') loadResources();
  });
  $('#btn-plan').addEventListener('click', loadPlan);
  $('#btn-resume-upload').addEventListener('click', uploadResume);
  // 成长 Chat 控件
  $('#btn-grow-send').addEventListener('click', growSend);
  $('#grow-input').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      growSend();
    }
  });
  $('#btn-grow-clear').addEventListener('click', growClear);
  // 历史记录：切换面板 / 收起
  $('#btn-grow-history').addEventListener('click', () => {
    if ($('#grow-history').style.display === 'none') {
      openGrowHistory();
    } else {
      closeGrowHistory();
    }
  });
  $('#btn-grow-history-close').addEventListener('click', closeGrowHistory);
  $('#btn-resume-text').addEventListener('click', saveResumeText);
  $('#btn-llm-test').addEventListener('click', () => testSettings('llm'));
  $('#btn-emb-test').addEventListener('click', () => testSettings('embedding'));
  $('#btn-settings-save').addEventListener('click', saveSettings);

  // --- 账户交互 ---
  // 标题栏：点击「登录 / 注册」打开登录视图；点击用户信息展开/收起下拉
  $('#btn-login').addEventListener('click', () => {
    switchAuthTab('login');
    showAuthOverlay(true);
    setTimeout(() => $('#login-email').focus(), 60);
  });
  $('#user-chip').addEventListener('click', (e) => {
    e.stopPropagation();
    $('#user-dropdown').classList.toggle('open');
  });
  document.addEventListener('click', (e) => {
    if (!e.target.closest('#account-area')) closeDropdown();
  });
  // 退出登录
  $('#btn-logout').addEventListener('click', async () => {
    try {
      await doLogout();
    } catch (e) { /* 忽略登出网络错误 */ }
    state.user = null;
    $('#user-dropdown').classList.remove('open');
    renderAccount();
    showAuthOverlay(true);
    switchAuthTab('login');
    toast('已退出登录');
  });

  // 登录 / 注册标签切换
  document.querySelectorAll('.auth-tab').forEach((b) =>
    b.addEventListener('click', () => switchAuthTab(b.dataset.authTab)));

  // 登录表单
  $('#auth-login').addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = $('#login-email').value.trim();
    const password = $('#login-password').value;
    if (!email || !password) return setAuthMessage('请输入邮箱和密码', true);
    setAuthMessage('登录中…');
    try {
      onAuthed(await doLogin(email, password));
      setAuthMessage('');
      $('#login-password').value = '';
      toast('登录成功');
    } catch (err) {
      setAuthMessage(errMsg(err), true);
    }
  });

  // 注册表单
  $('#auth-register').addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = $('#reg-name').value.trim();
    const email = $('#reg-email').value.trim();
    const password = $('#reg-password').value;
    if (!email) return setAuthMessage('请输入邮箱', true);
    if (password.length < 6) return setAuthMessage('密码至少 6 位', true);
    setAuthMessage('注册中…');
    try {
      onAuthed(await doRegister(email, password, name));
      setAuthMessage('');
      $('#reg-password').value = '';
      toast('注册成功，已自动登录');
    } catch (err) {
      setAuthMessage(errMsg(err), true);
    }
  });

  // --- 主题（深浅色） ---
  const THEME_KEY = 'mirror-mian-theme';
  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(THEME_KEY, theme);
  }
  const savedTheme = localStorage.getItem(THEME_KEY);
  if (savedTheme) {
    applyTheme(savedTheme);
  } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    applyTheme('dark');
  }
  $('#theme-toggle').addEventListener('click', () => {
    const cur = document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
    applyTheme(cur === 'dark' ? 'light' : 'dark');
  });

  // --- 启动：校验会话，未登录则展示登录遮罩 ---
  renderAccount(); // 初始状态（未登录）渲染标题栏
  showAuthOverlay(true); // 先锁屏，校验通过后再放开
  switchAuthTab('login');
  checkAuth()
    .then((user) => {
      if (user) {
        onAuthed(user); // 已登录：放开遮罩并加载数据
      } else {
        showAuthOverlay(true);
        setTimeout(() => $('#login-email').focus(), 60);
      }
    })
    .catch(() => {
      showAuthOverlay(true);
    });
})();
