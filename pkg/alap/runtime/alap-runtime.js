/**
 * Alap Web Enterprise Runtime v1.0
 * Provides client-side hydration, reactive state management, DOM reconciliation,
 * and real-time WebSocket / SSE live event binding.
 */
(function () {
  'use strict';

  console.log('⚡ [Alap Web] Initializing client-side runtime...');

  const initialState = window.__NILANG_INITIAL_STATE__ || {};
  const listeners = {};

  // Reactive State Store
  const state = new Proxy({ ...initialState }, {
    set(target, prop, value) {
      target[prop] = value;
      notifyPropertyChange(prop, value);
      return true;
    }
  });

  function notifyPropertyChange(prop, value) {
    // 1. Update bound text elements: [data-alap-text="prop"]
    document.querySelectorAll(`[data-alap-text="${prop}"]`).forEach(el => {
      el.textContent = value;
    });

    // 2. Update bound input values: [data-alap-bind="prop"]
    document.querySelectorAll(`[data-alap-bind="${prop}"]`).forEach(el => {
      if (el.value !== undefined && el.value !== String(value)) {
        el.value = value;
      }
    });

    // 3. Dispatch to registered event listeners
    if (listeners[prop]) {
      listeners[prop].forEach(fn => fn(value));
    }
  }

  // Event Bus
  function on(eventName, callback) {
    if (!listeners[eventName]) {
      listeners[eventName] = [];
    }
    listeners[eventName].push(callback);
  }

  function emit(eventName, payload) {
    if (listeners[eventName]) {
      listeners[eventName].forEach(fn => fn(payload));
    }
  }

  function syncState(nextState) {
    Object.keys(state).forEach(key => {
      if (!(key in nextState)) delete state[key];
    });
    Object.assign(state, nextState);
  }

  async function dispatchAction(action, payload) {
    const request = { event: action };
    if (payload !== undefined) request.payload = payload;
    const response = await fetch('/__alap/event', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });
    if (!response.ok) {
      const message = await response.text();
      throw new Error(`NilLang event ${action} failed: ${message}`);
    }

    const documentUpdate = new DOMParser().parseFromString(await response.text(), 'text/html');
    const currentRoot = document.querySelector('[data-alap-root]');
    const nextRoot = documentUpdate.querySelector('[data-alap-root]');
    if (!currentRoot || !nextRoot) {
      throw new Error('NilLang event response did not contain an Alap root');
    }
    currentRoot.replaceWith(document.importNode(nextRoot, true));

    const stateElement = documentUpdate.getElementById('__NILANG_STATE__');
    if (stateElement) syncState(JSON.parse(stateElement.textContent));
    hydrateDOM();
    emit('action:' + action, { state });
  }

  // DOM Event Hydration
  function hydrateDOM() {
    // Hydrate click handlers
    document.querySelectorAll('[data-alap-click]').forEach(el => {
      const action = el.getAttribute('data-alap-click');
      const payloadData = el.getAttribute('data-alap-payload');
      el.addEventListener('click', async () => {
        el.disabled = true;
        try {
          const payload = payloadData === null ? undefined : JSON.parse(payloadData);
          await dispatchAction(action, payload);
        } catch (error) {
          el.disabled = false;
          console.error(error);
        }
      });
    });

    // Hydrate two-way inputs
    document.querySelectorAll('[data-alap-bind]').forEach(el => {
      const prop = el.getAttribute('data-alap-bind');
      if (state[prop] !== undefined) {
        el.value = state[prop];
      }
      el.addEventListener('input', (e) => {
        state[prop] = e.target.value;
      });
    });

    console.log('✅ [Alap Web] Hydrated DOM nodes with reactive listeners');
  }

  // Real-time WebSocket connection
  function connectWebSocket(wsUrl) {
    const url = wsUrl || (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws/live';
    try {
      const ws = new WebSocket(url);
      ws.onopen = () => console.log('📡 [Alap Web] Real-time WebSocket connected');
      ws.onmessage = (msg) => {
        try {
          const data = JSON.parse(msg.data);
          if (data.type && data.payload) {
            emit(data.type, data.payload);
            if (data.type === 'state_update') {
              Object.assign(state, data.payload);
            }
          }
        } catch (e) {
          console.warn('[Alap Web] Non-JSON WS message:', msg.data);
        }
      };
      ws.onclose = () => {
        console.log('📡 [Alap Web] WS closed, reconnecting in 3s...');
        setTimeout(() => connectWebSocket(wsUrl), 3000);
      };
      return ws;
    } catch (err) {
      console.warn('⚠️ [Alap Web] WS connection failed:', err);
      return null;
    }
  }

  // Real-time Server-Sent Events (SSE)
  function connectSSE(sseUrl) {
    const url = sseUrl || '/events/live';
    if (!window.EventSource) return null;

    try {
      const es = new EventSource(url);
      es.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          emit('sse_message', data);
        } catch (_) {}
      };
      return es;
    } catch (e) {
      return null;
    }
  }

  // Public Alap Client API
  window.__ALAP__ = {
    state,
    on,
    emit,
    dispatch: dispatchAction,
    hydrate: hydrateDOM,
    connectWebSocket,
    connectSSE,
  };

  // Run initial hydration after DOM load
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', hydrateDOM);
  } else {
    hydrateDOM();
  }
})();
