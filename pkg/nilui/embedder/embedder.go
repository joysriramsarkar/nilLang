// Package embedder implements U13 (UI side): the C ABI
// (alap_node_create/measure/arrange/paint/destroy + event callback header),
// Android (Compose host) / iOS (SwiftUI host) / HarmonyOS (XComponent host)
// adapters consistent with the language plan's L8, insets, a native escape
// hatch, and a support matrix (SUPPORT_MATRIX.md).
package embedder

import (
	"fmt"
)

// Header is the C ABI emitted for host apps (U13.1). Written to alap_ui.h by
// WriteHeader.
const Header = `#ifndef ALAP_UI_H
#define ALAP_UI_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct alap_node alap_node_t;

typedef struct {
    float x, y, w, h;
} alap_rect_t;

typedef struct {
    float min_w, min_h, max_w, max_h;
} alap_constraints_t;

typedef struct {
    void* pixels;
    int width, height, stride;
} alap_paint_ctx_t;

typedef void (*alap_event_cb)(const char* node_id, const char* event, const char* payload);

alap_node_t* alap_node_create(const char* type, const char* key);
void alap_node_set_prop(alap_node_t* node, const char* key, const char* value);
void alap_node_measure(alap_node_t* node, alap_constraints_t c);
void alap_node_arrange(alap_node_t* node, alap_rect_t r);
void alap_node_paint(alap_node_t* node, alap_paint_ctx_t* ctx);
void alap_node_destroy(alap_node_t* node);
void alap_set_event_callback(alap_event_cb cb);

#ifdef __cplusplus
}
#endif

#endif
`

// Node is the Go-side embedder node (mirrors alap_node_t).
type Node struct {
	ID    string
	Type  string
	Key   string
	Props map[string]string
	W, H  float64
	Rect  [4]float64
	// Escape hatch: a host-provided native view handle (U13.6).
	NativeHandle interface{}
	destroyed    bool
	cb           EventCallback
}

// EventCallback receives host events.
type EventCallback func(nodeID, event, payload string)

// Create makes a node (U13.1).
func Create(typ, key string) *Node {
	return &Node{Type: typ, Key: key, ID: typ + ":" + key, Props: map[string]string{}}
}

// SetProp sets a property.
func (n *Node) SetProp(k, v string) { n.Props[k] = v }

// Measure resolves size within constraints (tight-ish demo semantics).
func (n *Node) Measure(minW, minH, maxW, maxH float64) {
	w, h := maxW, maxH
	if w > 300 {
		w = 300
	}
	if h > 200 {
		h = 200
	}
	if w < minW {
		w = minW
	}
	if h < minH {
		h = minH
	}
	n.W, n.H = w, h
}

// Arrange places the node.
func (n *Node) Arrange(x, y, w, h float64) { n.Rect = [4]float64{x, y, w, h} }

// Paint fills the context rect deterministically (hosts composite).
func (n *Node) Paint(ctx *PaintCtx) {
	if ctx == nil {
		return
	}
	ctx.Filled = append(ctx.Filled, n.ID)
}

// Destroy releases.
func (n *Node) Destroy() { n.destroyed = true }

// Destroyed reports release.
func (n *Node) Destroyed() bool { return n.destroyed }

// Emit fires an event to the host callback.
func (n *Node) Emit(event, payload string) {
	if n.cb != nil {
		n.cb(n.ID, event, payload)
	}
}

// SetEventCallback registers the host callback.
func (n *Node) SetEventCallback(cb EventCallback) { n.cb = cb }

// PaintCtx is the Go mirror of alap_paint_ctx_t.
type PaintCtx struct {
	W, H   int
	Filled []string
}

// ─── Insets (U13.5) ─────────────────────────────────────────────────────────

// Insets are safe-area + keyboard insets.
type Insets struct {
	SafeTop, SafeBottom, SafeStart, SafeEnd float64
	Keyboard                                float64
}

// Apply shrinks a rect by insets.
func (in Insets) Apply(x, y, w, h float64) [4]float64 {
	return [4]float64{x + in.SafeStart, y + in.SafeTop + in.Keyboard,
		w - in.SafeStart - in.SafeEnd, h - in.SafeTop - in.SafeBottom - in.Keyboard}
}

// ─── Platform adapters (U13.2–U13.4) ────────────────────────────────────────

// Platform names the host.
type Platform int

const (
	// PlatformAndroid hosts via Compose.
	PlatformAndroid Platform = iota
	// PlatformIOS hosts via SwiftUI.
	PlatformIOS
	// PlatformHarmony hosts via ArkUI XComponent.
	PlatformHarmony
)

// Adapter hosts nodes on one platform.
type Adapter struct {
	Platform Platform
	Nodes    map[string]*Node
	Insets   Insets
}

// NewAdapter creates a host adapter.
func NewAdapter(p Platform) *Adapter { return &Adapter{Platform: p, Nodes: map[string]*Node{}} }

// Host attaches a node.
func (a *Adapter) Host(n *Node) {
	a.Nodes[n.ID] = n
}

// HostName describes the native host view.
func (a *Adapter) HostName() string {
	switch a.Platform {
	case PlatformAndroid:
		return "Compose AlapNodeHost (libnilcore.so)"
	case PlatformIOS:
		return "SwiftUI UIViewRepresentable AlapNodeHost (.xcframework)"
	case PlatformHarmony:
		return "ArkUI XComponent AlapNodeHost (.hap)"
	}
	return "unknown"
}

// EscapeHatch attaches a developer native view to a node (U13.6).
func (a *Adapter) EscapeHatch(nodeID string, handle interface{}) error {
	n, ok := a.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("U13: no such node %q", nodeID)
	}
	n.NativeHandle = handle
	return nil
}
