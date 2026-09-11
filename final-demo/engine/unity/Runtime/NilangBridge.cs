using System.Runtime.InteropServices;

namespace Nilang.Unity
{
    public static class NilangBridge
    {
        public static void Tick(float deltaSeconds)
        {
            // Native plugin entry point is intentionally optional while authoring in the editor.
        }
    }
}
