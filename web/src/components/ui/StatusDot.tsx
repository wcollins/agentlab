import { cn } from '../../lib/cn';
import type { NodeStatus } from '../../types';

interface StatusDotProps {
  status: NodeStatus;
}

const dotColors: Record<NodeStatus, string> = {
  running: 'bg-status-running',
  stopped: 'bg-status-stopped',
  error: 'bg-status-error',
  initializing: 'bg-status-pending',
};

export function StatusDot({ status }: StatusDotProps) {
  return (
    <span
      className={cn(
        'rounded-full w-1.5 h-1.5', // 6px size
        dotColors[status]
      )}
    />
  );
}
