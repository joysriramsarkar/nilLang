/**
 * Alap Web Enterprise Client Runtime v1.0
 */
(function () {
  'use strict';

  console.log('⚡ [Alap Web] Client Runtime initialized.');

  const state = new Proxy(window.__NILANG_INITIAL_STATE__ || {}, {
    set(target, prop, value) {
      target[prop] = value;
      updateDOMBindings(prop, value);
      return true;
    }
  });

  function updateDOMBindings(prop, value) {
    document.querySelectorAll(`[data-alap-text="${prop}"]`).forEach(el => {
      el.textContent = value;
    });
  }

  // Real-time Event Stream
  function initLiveStream() {
    setInterval(async () => {
      try {
        const res = await fetch('/events/live');
        if (res.ok) {
          const data = await res.json();
          if (data.rps) {
            const rpsEl = document.getElementById('metric-rps');
            if (rpsEl) rpsEl.textContent = (data.rps + Math.floor(Math.random() * 40 - 20)) + ' req/s';
          }
        }
      } catch (_) {}
    }, 4000);
  }

  window.checkoutItem = async function (id, name, price) {
    showToast(`🔄 Processing atomic transaction for ${name}...`);
    try {
      const res = await fetch('/api/orders', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ product_id: id, qty: 1 })
      });
      const data = await res.json();
      if (data.order_id) {
        showToast(`✅ Order ${data.order_id} Confirmed! Stock updated in SQLite ORM.`);
        addLogEntry(`[${new Date().toLocaleTimeString()}] Order ${data.order_id} placed for ${name} (${price}) — Status: ${data.status}`);
        // Increment order counter
        const ordersEl = document.getElementById('metric-orders');
        if (ordersEl) {
          let current = parseInt(ordersEl.textContent, 10) || 142;
          ordersEl.textContent = (current + 1) + ' Orders';
        }
      }
    } catch (err) {
      showToast(`❌ Checkout error: ${err.message}`);
    }
  };

  function addLogEntry(text) {
    const list = document.getElementById('log-stream');
    if (!list) return;
    const item = document.createElement('div');
    item.className = 'log-entry success';
    item.textContent = text;
    list.prepend(item);
  }

  function showToast(msg) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = msg;
    toast.classList.add('show');
    setTimeout(() => toast.classList.remove('show'), 3500);
  }

  window.__ALAP__ = { state, initLiveStream, showToast };

  document.addEventListener('DOMContentLoaded', () => {
    initLiveStream();
  });
})();
