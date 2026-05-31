import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// jsdom keeps state across tests inside the same file unless we tell it
// to wipe between cases. Without this, sessionStorage from test 1
// leaks into test 2 and the auth tests fail in mysterious ways.
afterEach(() => {
  cleanup();
  sessionStorage.clear();
});
