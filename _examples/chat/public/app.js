(function () {
  let timeoutId = null;
  function appendMessage(message, me) {
    const chat = document.getElementById('chat');
    let cardClass = 'chat-card hide';
    if (me) {
      cardClass += ' me';
    }

    const li = document.createElement('li');
    li.className = cardClass;

    li.innerHTML = `
    <h2 class="chat-card__sender">${message.sender}</h2>
    <p class="chat-card__text">${message.text}</p>
    `;

    chat.appendChild(li);
    chat.scroll(0, chat.scrollHeight);
    setTimeout(() => li.classList.remove('hide'), 200);
  }

  function appendNotification(text) {
    const notification = document.getElementById('notification');
    notification.innerHTML = null;
    const p = document.createElement('p');
    p.innerText = text;
    notification.classList.add('show');
    notification.appendChild(p);

    clearTimeout(timeoutId);

    timeoutId = setTimeout(() => {
      notification.classList.remove('show');
      notification.innerHTML = null;
    }, 3000);
  }

  const nickname = prompt('what is your nickname?', '');
  const ws = new WebSocket(`ws://localhost:8000/ws?nickname=${nickname}`);

  ws.addEventListener('open', () => {
    document.body.style.backgroundColor = '#1B2224';
  });

  ws.addEventListener('message', e => {
    const message = JSON.parse(e.data);
    if (message.type === 2) {
      appendMessage(message, false);
    } else if (message.type === 1) {
      appendNotification(message.text);
    }
  });

  const text = document.getElementById('text');
  const form = document.getElementById('form');

  form.addEventListener('submit', e => {
    e.preventDefault();
    const value = text.value.trim();

    if (value === '') {
      return;
    }

    const message = {
      type: 2,
      text: value,
      sender: nickname
    };

    ws.send(JSON.stringify(message));
    text.value = '';
    appendMessage(message, true);
  });

  text.addEventListener('keydown', e => {
    if (['Enter', 'Backspace', 'Delete', 'Shift'].includes(e.key)) {
      return
    }

    const message = {
      type: 1
    };

    ws.send(JSON.stringify(message));
  });
})();
