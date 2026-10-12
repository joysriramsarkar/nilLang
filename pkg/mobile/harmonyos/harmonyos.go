// Package harmonyos scaffolds the Nilang HarmonyOS embedder (L8.5): a NAPI
// bridge plus an ArkTS application stub packaged as a `.hap` manifest.
package harmonyos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NAPIBridgeC is the Node-API stub compiled into the HarmonyOS native
// layer. It forwards bundle execution to the nil_runtime.h C ABI.
const NAPIBridgeC = `#include <node_api.h>
#include "nil_runtime.h"

// runBundle(data: ArrayBuffer) -> int32 — entry used by the ArkTS layer.
static napi_value RunBundle(napi_env env, napi_callback_info info) {
    nil_ffi_abi_v1();
    size_t argc = 1;
    napi_value args[1];
    napi_get_cb_info(env, info, &argc, args, NULL, NULL);
    int32_t rc = nil_init();
    if (rc != 0) {
        napi_value result;
        napi_create_int32(env, rc, &result);
        return result;
    }
    napi_value result;
    napi_create_int32(env, 0, &result);
    return result;
}

napi_value Init(napi_env env, napi_value exports) {
    napi_property_descriptor desc = {"runBundle", 0, RunBundle, 0, 0, 0, napi_default, 0};
    napi_define_properties(env, exports, 1, &desc);
    return exports;
}

NAPI_MODULE(nilang_core, Init)
`

// ArkTSEntry renders the ArkTS ability stub that boots the Nilang bundle.
func ArkTSEntry(appName string) string {
	return fmt.Sprintf(`// EntryAbility.ets — HarmonyOS host for %s (L8.5).
// The .nilax bundle ships in resources/rawfile/app.nilax; calls cross
// through the NAPI bridge (libnilang_core.so) into nil_runtime.h.
import hilog from '@ohos.hilog';

export default class EntryAbility {
  onCreate() {
    hilog.info(0x0000, 'nilang', '%s booting via NAPI bridge');
  }
  onForeground() {}
  onBackground() {}
  onDestroy() {}
}
`, appName, appName)
}

// HapManifest renders the .hap module.json stub.
func HapManifest(appName, version string) string {
	return fmt.Sprintf(`{
  "app": {"bundleName": "dev.nilang.%s", "versionCode": 1, "versionName": %q},
  "module": {
    "name": "entry",
    "type": "entry",
    "abilities": [{"name": "EntryAbility", "srcEntry": "./ets/EntryAbility.ets"}],
    "requestPermissions": []
  }
}
`, strings.ToLower(appName), version)
}

// Scaffold writes the HarmonyOS app stub into dir.
func Scaffold(dir, appName, version string) error {
	files := map[string]string{
		"entry/src/main/cpp/nilang_bridge.cpp":                   NAPIBridgeC,
		"entry/src/main/ets/EntryAbility.ets":                    ArkTSEntry(appName),
		"entry/module.json":                                      HapManifest(appName, version),
		"entry/src/main/resources/rawfile/app.nilax.PLACEHOLDER": "bundle copied here by: nil build harmony\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
