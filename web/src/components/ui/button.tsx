import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { clsx } from 'clsx';

type ButtonProps = React.ComponentProps<'button'> & {
  asChild?: boolean;
  variant?: 'default' | 'outline' | 'ghost' | 'destructive';
  size?: 'default' | 'compact' | 'touch';
  loading?: boolean;
};
export function Button({ className, variant='default', size='default', asChild=false, loading=false, style, children, disabled, ...props }: ButtonProps){
  const Component = asChild ? Slot : 'button';
  return <Component className={clsx('ui-button inline-flex items-center justify-center gap-2 px-3 font-medium transition-colors',
    variant==='default' && 'ui-button-primary',
    variant==='outline' && 'ui-button-outline',
    variant==='ghost' && 'ui-button-ghost',
    variant==='destructive' && 'ui-button-danger',
    `ui-button-${size}`, className)} style={style} disabled={disabled||loading} aria-busy={loading||undefined} {...props}>{children}</Component>;
}
