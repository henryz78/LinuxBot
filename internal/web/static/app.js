const state = { sessionId: null };

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
  }
}

function appendMessage(answer, summary, runId) {
  const container = document.querySelector('#messages');
  const article = document.createElement('article');
  article.className = 'message';

  const text = document.createElement('p');
  text.textContent = answer;

  const details = document.createElement('details');
  const summaryNode = document.createElement('summary');
  summaryNode.textContent = summary;
  const pre = document.createElement('pre');
  details.appendChild(summaryNode);
  details.appendChild(pre);
  details.addEventListener('toggle', async () => {
    if (details.open && pre.textContent === '') {
      const response = await fetch(`/api/runs/${runId}`);
      const steps = await response.json();
      pre.textContent = JSON.stringify(steps, null, 2);
      for (const step of steps) {
        if (step.Status === 'approval_required') {
          const button = document.createElement('button');
          button.type = 'button';
          button.className = 'secondary';
          button.textContent = '批准执行';
          button.addEventListener('click', async () => {
            await fetch('/api/approve', {
              method: 'POST',
              headers: {'Content-Type': 'application/json'},
              body: JSON.stringify({session_id: state.sessionId, step_id: step.ID})
            });
            pre.textContent = '';
            details.open = false;
          });
          details.appendChild(button);
        }
      }
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
      body: JSON.stringify({session_id: state.sessionId, run_id: runId})
    });
    article.remove();
  });
  actions.appendChild(deleteButton);

  article.appendChild(text);
  article.appendChild(details);
  article.appendChild(actions);
  container.appendChild(article);
  container.scrollTop = container.scrollHeight;
}

document.querySelector('#session').addEventListener('change', event => {
  state.sessionId = Number(event.target.value);
  const option = event.target.selectedOptions[0];
  if (option) {
    document.querySelector('#mode').value = option.dataset.mode;
  }
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
    appendMessage(data.answer, data.summary, data.run_id);
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
