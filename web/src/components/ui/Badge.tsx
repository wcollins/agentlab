import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';
import type { NodeStatus } from '../../types';

interface BadgeProps {
  status: NodeStatus;
  children: ReactNode;
  className?: string;
}

const statusStyles: Record<NodeStatus, string> = {
  running: 'bg-status-running/10 text-status-running border-status-running/30',
  stopped: 'bg-status-stopped/20 text-text-muted border-status-stopped/40',
  error: 'bg-status-error/10 text-status-error border-status-error/30',
  initializing: 'bg-status-pending/10 text-status-pending border-status-pending/30',
};

export function Badge({ status, children, className }: BadgeProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded border',
        'text-[10px] uppercase font-medium tracking-wider',
        statusStyles[status],
        className
      )}
    >
      {children}
    </div>
  );
}
