using UnityEngine;

namespace Nilang.Unity
{
    public sealed class NilangBehaviour : MonoBehaviour
    {
        [SerializeField] private TextAsset bundle;

        private void Start()
        {
            var bundleName = bundle != null ? bundle.name : "StreamingAssets/nilang/app.nilax";
            Debug.Log("Nilang bundle ready: " + bundleName);
        }

        private void Update()
        {
            NilangBridge.Tick(Time.deltaTime);
        }
    }
}
