import { Router } from '@asymmetric-effort/specifyjs';

import { Layout } from './components/Layout';
import { PageRouter } from './components/PageRouter';

export function App() {
  return (
    <Router>
      <Layout>
        <PageRouter />
      </Layout>
    </Router>
  );
}
