package ui

// AlapRuntimeJS is a lightweight, zero-dependency browser JavaScript engine
// that performs in-place Incremental DOM patching and event delegation.
const AlapRuntimeJS = `
/**
 * Alap UI Browser Runtime & Incremental DOM Patcher
 * Lightweight Virtual DOM Hydration & Fine-grained Patching
 */
(function(window) {
    'use strict';

    const AlapRuntime = {
        version: "1.0.0",

        /**
         * Resolves DOM node from path array e.g. [0, 2, 1]
         */
        resolveNodeByPath: function(root, path) {
            let current = root;
            for (let i = 0; i < path.length; i++) {
                if (!current || !current.childNodes || current.childNodes.length <= path[i]) {
                    return null;
                }
                current = current.childNodes[path[i]];
            }
            return current;
        },

        /**
         * Creates a real DOM element from a VNode object
         */
        createDOMNode: function(vnode) {
            if (!vnode) return document.createTextNode("");
            if (vnode.type === "TEXT") {
                return document.createTextNode(vnode.text || "");
            }

            const el = document.createElement(vnode.tag || "div");
            if (vnode.id) el.id = vnode.id;
            if (vnode.key) el.setAttribute("data-key", vnode.key);

            if (vnode.attrs) {
                for (const [k, v] of Object.entries(vnode.attrs)) {
                    if (k === "class") {
                        el.className = v;
                    } else if (k === "style") {
                        el.style.cssText = v;
                    } else {
                        el.setAttribute(k, v);
                    }
                }
            }

            if (vnode.events) {
                for (const [evt, handler] of Object.entries(vnode.events)) {
                    el.setAttribute("data-alap-" + evt, handler);
                }
            }

            if (vnode.children) {
                for (let i = 0; i < vnode.children.length; i++) {
                    el.appendChild(this.createDOMNode(vnode.children[i]));
                }
            }
            return el;
        },

        /**
         * Applies an array of incremental patches to the live DOM
         */
        applyPatches: function(rootElement, patches) {
            if (!rootElement || !Array.isArray(patches)) return;

            for (let i = 0; i < patches.length; i++) {
                const patch = patches[i];
                let target = null;

                if (patch.node_id) {
                    target = document.getElementById(patch.node_id);
                }
                if (!target && patch.path) {
                    target = this.resolveNodeByPath(rootElement, patch.path);
                }
                if (!target) target = rootElement;

                switch (patch.type) {
                    case "REPLACE": {
                        const newEl = this.createDOMNode(patch.vnode);
                        if (target.parentNode) {
                            target.parentNode.replaceChild(newEl, target);
                        }
                        break;
                    }
                    case "TEXT": {
                        target.textContent = patch.text || "";
                        break;
                    }
                    case "PROPS": {
                        if (patch.props && target.nodeType === 1) {
                            for (const [k, v] of Object.entries(patch.props)) {
                                if (v === "") {
                                    target.removeAttribute(k);
                                } else if (k === "class") {
                                    target.className = v;
                                } else if (k === "style") {
                                    target.style.cssText = v;
                                } else {
                                    target.setAttribute(k, v);
                                }
                            }
                        }
                        break;
                    }
                    case "INSERT_CHILD": {
                        const newChild = this.createDOMNode(patch.vnode);
                        target.appendChild(newChild);
                        break;
                    }
                    case "REMOVE_CHILD": {
                        if (target.childNodes && target.childNodes.length > patch.index) {
                            target.removeChild(target.childNodes[patch.index]);
                        }
                        break;
                    }
                }
            }
        },

        /**
         * Setup global event delegation for Alap UI interactive events
         */
        initEventDelegation: function(rootElement, dispatchCallback) {
            const events = ["click", "input", "change", "submit"];
            events.forEach(eventType => {
                rootElement.addEventListener(eventType, function(e) {
                    let target = e.target;
                    while (target && target !== rootElement) {
                        const handler = target.getAttribute("data-alap-" + eventType);
                        if (handler) {
                            const payloadAttr = target.getAttribute("data-alap-payload");
                            let payload = null;
                            if (payloadAttr) {
                                try { payload = JSON.parse(payloadAttr); } catch (err) { payload = payloadAttr; }
                            }
                            if (typeof dispatchCallback === "function") {
                                dispatchCallback({
                                    event: eventType,
                                    handler: handler,
                                    targetId: target.id,
                                    value: target.value !== undefined ? target.value : null,
                                    payload: payload
                                });
                            }
                            break;
                        }
                        target = target.parentNode;
                    }
                });
            });
        }
    };

    window.AlapRuntime = AlapRuntime;
})(window);
`
