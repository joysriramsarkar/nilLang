/**
 * Alap UI Unified Browser Runtime v2.0 (ADR-0005: single keyed-patch runtime)
 *
 * This is the ONLY client runtime served by nil dev (see /alap-runtime.js).
 * It combines:
 *  - keyed incremental DOM patching (applyPatches: REPLACE/TEXT/PROPS/
 *    INSERT_CHILD/REMOVE_CHILD/MOVE), mirroring Go pkg/alap/ui Diff,
 *  - reactive state store with data-alap-text / data-alap-bind hydration,
 *  - event delegation that POSTs to /__alap/event and applies PATCH responses.
 */

(function (window, document) {
  'use strict';

  console.log('[Alap UI] Initializing unified keyed-patch runtime v2.0...');

  var initialState = window.__NILANG_INITIAL_STATE__ || {};
  var listeners = {};

  var state = new Proxy(Object.assign({}, initialState), {
    set: function (target, prop, value) {
      target[prop] = value;
      notifyPropertyChange(prop, value);
      return true;
    }
  });

  function notifyPropertyChange(prop, value) {
    document.querySelectorAll('[data-alap-text="' + prop + '"]').forEach(function (el) {
      el.textContent = value;
    });
    document.querySelectorAll('[data-alap-bind="' + prop + '"]').forEach(function (el) {
      if (el.value !== undefined && el.value !== String(value)) el.value = value;
    });
    if (listeners[prop]) listeners[prop].forEach(function (fn) { fn(value); });
  }

  function on(eventName, callback) {
    if (!listeners[eventName]) listeners[eventName] = [];
    listeners[eventName].push(callback);
  }

  function emit(eventName, payload) {
    if (listeners[eventName]) listeners[eventName].forEach(function (fn) { fn(payload); });
  }

  function syncState(nextState) {
    Object.keys(state).forEach(function (key) {
      if (!(key in nextState)) delete state[key];
    });
    Object.assign(state, nextState);
  }

  function resolveNodeByPath(root, path) {
    var current = root;
    for (var i = 0; i < path.length; i++) {
      if (!current || !current.childNodes || current.childNodes.length <= path[i]) return null;
      current = current.childNodes[path[i]];
    }
    return current;
  }

  function createDOMNode(vnode) {
    if (!vnode) return document.createTextNode('');
    if (vnode.type === 'TEXT') return document.createTextNode(vnode.text || '');
    var el = document.createElement(vnode.tag || 'div');
    if (vnode.id) el.id = vnode.id;
    if (vnode.key) el.setAttribute('data-key', vnode.key);
    if (vnode.attrs) {
      Object.entries(vnode.attrs).forEach(function (kv) {
        var k = kv[0], v = kv[1];
        if (k === 'class') el.className = v;
        else if (k === 'style') el.style.cssText = v;
        else el.setAttribute(k, v);
      });
    }
    if (vnode.events) {
      Object.entries(vnode.events).forEach(function (kv) {
        el.setAttribute('data-alap-' + kv[0], kv[1]);
      });
    }
    if (vnode.children) {
      for (var i = 0; i < vnode.children.length; i++) el.appendChild(createDOMNode(vnode.children[i]));
    }
    return el;
  }

  function applyPatches(rootElement, patches) {
    if (!rootElement || !Array.isArray(patches)) return;
    for (var i = 0; i < patches.length; i++) {
      var patch = patches[i];
      var target = null;
      if (patch.node_id) target = document.getElementById(patch.node_id);
      if (!target && patch.path) target = resolveNodeByPath(rootElement, patch.path);
      if (!target) target = rootElement;
      switch (patch.type) {
        case 'REPLACE': {
          var newEl = createDOMNode(patch.vnode);
          if (target.parentNode) target.parentNode.replaceChild(newEl, target);
          break;
        }
        case 'TEXT':
          target.textContent = patch.text || '';
          break;
        case 'PROPS':
          if (patch.props && target.nodeType === 1) {
            Object.entries(patch.props).forEach(function (kv) {
              if (kv[1] === '') target.removeAttribute(kv[0]);
              else if (kv[0] === 'class') target.className = kv[1];
              else if (kv[0] === 'style') target.style.cssText = kv[1];
              else target.setAttribute(kv[0], kv[1]);
            });
          }
          break;
        case 'INSERT_CHILD':
          target.insertBefore(
            createDOMNode(patch.vnode),
            target.childNodes[patch.index] || null
          );
          break;
        case 'REMOVE_CHILD':
          if (target.childNodes && target.childNodes.length > patch.index) {
            target.removeChild(target.childNodes[patch.index]);
          }
          break;
        case 'MOVE': {
          if (target.childNodes && target.childNodes.length > patch.from) {
            var moved = target.childNodes[patch.from];
            target.removeChild(moved);
            target.insertBefore(moved, target.childNodes[patch.to] || null);
          }
          break;
        }
      }
    }
  }

  function applyPatchResponse(payload) {
    // Server may answer an event with {patches, state} (preferred, ADR-0005)
    // or with a full HTML document (legacy fallback).
    if (payload && Array.isArray(payload.patches)) {
      var root = document.querySelector('[data-alap-root]');
      applyPatches(root, payload.patches);
      if (payload.state) syncState(payload.state);
      hydrateDOM();
      return true;
    }
    return false;
  }

  function dispatchAction(action, payload) {
    var request = { event: action };
    if (payload !== undefined) request.payload = payload;
    return fetch('/__alap/event', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json, text/html' },
      body: JSON.stringify(request)
    }).then(function (response) {
      if (!response.ok) throw new Error('NilLang event ' + action + ' failed: ' + response.status);
      var contentType = response.headers.get('Content-Type') || '';
      if (contentType.indexOf('application/json') !== -1) {
        return response.json().then(function (data) {
          if (!applyPatchResponse(data)) throw new Error('Unknown patch response');
          emit('action:' + action, { state: state });
        });
      }
      // Legacy HTML fallback: full-root replace (deprecated path).
      return response.text().then(function (html) {
        var doc = new DOMParser().parseFromString(html, 'text/html');
        var currentRoot = document.querySelector('[data-alap-root]');
        var nextRoot = doc.querySelector('[data-alap-root]');
        if (!currentRoot || !nextRoot) throw new Error('Event response missing Alap root');
        currentRoot.replaceWith(document.importNode(nextRoot, true));
        var stateElement = doc.getElementById('__NILANG_STATE__');
        if (stateElement) syncState(JSON.parse(stateElement.textContent));
        hydrateDOM();
        emit('action:' + action, { state: state });
      });
    });
  }

  function hydrateDOM() {
    document.querySelectorAll('[data-alap-click]').forEach(function (el) {
      if (el.__alapHydrated) return;
      el.__alapHydrated = true;
      var action = el.getAttribute('data-alap-click');
      var payloadData = el.getAttribute('data-alap-payload');
      el.addEventListener('click', function () {
        el.disabled = true;
        var payload = payloadData === null ? undefined : JSON.parse(payloadData);
        dispatchAction(action, payload).catch(function (err) {
          el.disabled = false;
          console.error(err);
        });
      });
    });
    document.querySelectorAll('[data-alap-bind]').forEach(function (el) {
      if (el.__alapHydrated) return;
      el.__alapHydrated = true;
      var prop = el.getAttribute('data-alap-bind');
      if (state[prop] !== undefined) el.value = state[prop];
      el.addEventListener('input', function (e) { state[prop] = e.target.value; });
    });
  }

  window.AlapRuntime = {
    version: '2.0.0',
    state: state,
    on: on,
    emit: emit,
    syncState: syncState,
    resolveNodeByPath: resolveNodeByPath,
    createDOMNode: createDOMNode,
    applyPatches: applyPatches,
    dispatch: dispatchAction,
    hydrate: hydrateDOM
  };
  window.__ALAP__ = window.AlapRuntime;

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', hydrateDOM);
  else hydrateDOM();
})(window, document);
