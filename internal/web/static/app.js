const state = { sessionId: null };

function runId(run) {
  return run.id || run.run_id || run.ID;
}

function sessionId(run) {
  return run.session_id || run.SessionID || state.sessionId;
}

async function loadSessions() {
  const response = await fetch('/api/sessions');
  const sessions = await response.json();
  const select = document.querySelector('#session');
  select.innerHTML = '';
  for (const session of sessions) {
    const option = document.createElement('option');
    option.value = session.ID;
    option.textContent = `${session.Name} / ${session.Mode}`;
    option.dataset.name = session.Name;
    option.dataset.mode = session.Mode;
    select.appendChild(option);
  }
  if (sessions.length > 0) {
    state.sessionId = sessions[0].ID;
    document.querySelector('#mode').value = sessions[0].Mode;
    await loadHistory();
  }
}

async function loadHistory() {
  const container = document.querySelector('#messages');
  container.innerHTML = '';
  if (!state.sessionId) {
    return;
  }
  const response = await fetch(`/api/history?session_id=${state.sessionId}`);
  const runs = await response.json();
  for (const run of runs) {
    appendRun(run);
  }
}

function appendRun(run) {
  const container = document.querySelector('#messages');
  const group = document.createElement('section');
  group.className = 'run';
  group.dataset.runId = runId(run);

  group.appendChild(createUserMessage(run.prompt || run.Prompt || ''));
  group.appendChild(createAssistantMessage(run, group));
  container.appendChild(group);
  container.scrollTop = container.scrollHeight;
}

function createUserMessage(prompt) {
  const article = document.createElement('article');
  article.className = 'message user';
  const label = document.createElement('div');
  label.className = 'message-label';
  label.textContent = 'You';
  const text = document.createElement('p');
  text.textContent = prompt;
  article.appendChild(label);
  article.appendChild(text);
  return article;
}

function createAssistantMessage(run, group) {
  const article = document.createElement('article');
  article.className = 'message assistant';

  const label = document.createElement('div');
  label.className = 'message-label';
  label.textContent = 'LinuxBot';

  const text = document.createElement('p');
  text.textContent = run.answer || run.Answer || '处理中。';

  const details = document.createElement('details');
  const summaryNode = document.createElement('summary');
  summaryNode.textContent = run.summary || run.Summary || '已处理';
  const trace = document.createElement('div');
  trace.className = 'trace';
  details.appendChild(summaryNode);
  details.appendChild(trace);
  details.addEventListener('toggle', async () => {
    if (details.open && trace.childElementCount === 0) {
      await loadRunDetails(run, trace, group, article);
    }
  });

  const actions = document.createElement('div');
  actions.className = 'message-actions';
  const deleteButton = document.createElement('button');
  deleteButton.type = 'button';
  deleteButton.className = 'secondary danger';
  deleteButton.textContent = '删除';
  deleteButton.addEventListener('click', async () => {
    await fetch('/api/delete-run', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({session_id: sessionId(run), run_id: runId(run)})
    });
    group.remove();
  });
  actions.appendChild(deleteButton);

  article.appendChild(label);
  article.appendChild(text);
  article.appendChild(details);
  article.appendChild(actions);
  return article;
}

async function loadRunDetails(run, trace, group, article) {
  const response = await fetch(`/api/runs/${runId(run)}?session_id=${sessionId(run)}`);
  const steps = await response.json();
  renderSteps(steps, trace, group, article);
}

function renderSteps(steps, trace, group, article) {
  trace.innerHTML = '';
  for (const step of steps) {
    const item = document.createElement('section');
    item.className = 'step';

    const title = document.createElement('div');
    title.className = `step-title status-${step.Status}`;
    title.textContent = `${stepLabel(step.Kind)} · ${step.Status}`;
    item.appendChild(title);

    if (step.Input) {
      const code = document.createElement('code');
      code.textContent = step.Input;
      item.appendChild(code);
    }
    const output = step.Output || step.ErrorText;
    if (output) {
      const pre = document.createElement('pre');
      pre.textContent = output;
      item.appendChild(pre);
    }
    if (step.Status === 'approval_required') {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'secondary';
      button.textContent = '批准执行';
      button.addEventListener('click', async () => {
        button.disabled = true;
        const response = await fetch('/api/approve', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({session_id: state.sessionId, step_id: step.ID})
        });
        if (!response.ok) {
          button.disabled = false;
          return;
        }
        const updatedRun = await response.json();
        const next = createAssistantMessage(updatedRun, group);
        article.replaceWith(next);
      });
      item.appendChild(button);
    }
    trace.appendChild(item);
  }
}

function stepLabel(kind) {
  switch (kind) {
    case 'plan':
      return '计划';
    case 'command':
      return '命令';
    case 'search':
      return '搜索';
    case 'answer':
      return '回答';
    default:
      return kind || '步骤';
  }
}

document.querySelector('#session').addEventListener('change', async event => {
  state.sessionId = Number(event.target.value);
  const option = event.target.selectedOptions[0];
  if (option) {
    document.querySelector('#mode').value = option.dataset.mode;
  }
  await loadHistory();
});

document.querySelector('#mode').addEventListener('change', async event => {
  if (!state.sessionId) {
    return;
  }
  await fetch('/api/mode', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({session_id: state.sessionId, mode: event.target.value})
  });
  const selected = document.querySelector('#session').selectedOptions[0];
  if (selected) {
    selected.dataset.mode = event.target.value;
    selected.textContent = `${selected.dataset.name} / ${event.target.value}`;
  }
});

document.querySelector('#ask').addEventListener('submit', async event => {
  event.preventDefault();
  const promptNode = document.querySelector('#prompt');
  const prompt = promptNode.value.trim();
  if (!prompt || !state.sessionId) {
    return;
  }
  const button = event.submitter;
  button.disabled = true;
  try {
    const response = await fetch('/api/ask', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({session_id: state.sessionId, prompt})
    });
    const data = await response.json();
    appendRun(data.run || {
      id: data.run_id,
      session_id: state.sessionId,
      prompt,
      answer: data.answer,
      summary: data.summary,
      status: data.status || 'done'
    });
    promptNode.value = '';
  } finally {
    button.disabled = false;
  }
});

async function boot() {
  const response = await fetch('/api/health');
  const data = await response.json();
  document.querySelector('#connection').textContent = data.ok ? 'Local only: 127.0.0.1' : 'Offline';
  await loadSessions();
}

boot();
