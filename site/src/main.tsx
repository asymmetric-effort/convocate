import { createRoot } from '@asymmetric-effort/specifyjs/dom';

import { App } from './app';
import './styles.css';

const container = document.getElementById('root');
if (!container) {
  throw new Error('site: #root element missing from index.html');
}

createRoot(container).render(<App />);
