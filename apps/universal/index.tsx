import { ExpoRoot } from 'expo-router';
import { registerRootComponent } from 'expo';

// Work around Metro/app-root resolution issues in workspace layouts by
// providing a stable explicit app context for expo-router.
export function App() {
  const ctx = require.context('./app');
  return <ExpoRoot context={ctx} />;
}

registerRootComponent(App);
