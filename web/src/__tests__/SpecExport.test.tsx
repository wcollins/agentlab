import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { SpecTab } from '../components/spec/SpecTab';
import { useSpecStore } from '../stores/useSpecStore';
import { fetchStackExport } from '../lib/api';

vi.mock('../lib/api', () => ({
  fetchStackSpec: vi.fn().mockResolvedValue({ path: '/stack.yaml', content: 'name: raw-authored-spec' }),
  fetchStackHealth: vi.fn().mockResolvedValue({}),
  validateStackSpec: vi.fn().mockResolvedValue({ valid: true, issues: [] }),
  fetchStackExport: vi.fn(),
}));

beforeEach(() => {
  useSpecStore.setState({ spec: null, specError: null, specLoading: false, validation: null });
  vi.stubGlobal('URL', Object.assign(URL, {
    createObjectURL: vi.fn().mockReturnValue('blob:export'),
    revokeObjectURL: vi.fn(),
  }));
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('downloads only the API export and keeps raw content unchanged', async () => {
  vi.mocked(fetchStackExport).mockResolvedValue({ content: 'name: exported', format: 'yaml', notice: 'Review authored literals before sharing.' });
  render(<SpecTab />);
  const button = await screen.findByRole('button', { name: 'Export YAML' });
  expect(button).toHaveAttribute('aria-describedby', 'stack-export-notice');
  button.focus();
  expect(button).toHaveFocus();
  fireEvent.click(button);
  await waitFor(() => expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledOnce());
  expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
  expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:export');
  expect(screen.getByText(/Review authored literals before sharing/)).toBeInTheDocument();
  expect(useSpecStore.getState().spec?.content).toBe('name: raw-authored-spec');
  expect(screen.getByText(/raw-authored-spec/)).toBeInTheDocument();
});

it('prevents repeated actions and downloads on failure, then permits retry', async () => {
  let rejectExport!: (reason: Error) => void;
  vi.mocked(fetchStackExport).mockReturnValue(new Promise((_, reject) => { rejectExport = reject; }));
  render(<SpecTab />);
  fireEvent.click(await screen.findByRole('button', { name: 'Export YAML' }));
  const pending = screen.getByRole('button', { name: 'Exporting...' });
  expect(pending).toBeDisabled();
  fireEvent.click(pending);
  expect(fetchStackExport).toHaveBeenCalledOnce();
  rejectExport(new Error('export: gateway.auth.token: recognized sensitive field contains an inline literal'));
  expect(await screen.findByRole('alert')).toHaveTextContent('gateway.auth.token');
  expect(URL.createObjectURL).not.toHaveBeenCalled();
  expect(HTMLAnchorElement.prototype.click).not.toHaveBeenCalled();
  expect(screen.getByRole('button', { name: 'Export YAML' })).toBeEnabled();
  expect(useSpecStore.getState().spec?.content).toBe('name: raw-authored-spec');
});
