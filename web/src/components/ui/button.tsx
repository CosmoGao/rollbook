import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { clsx } from 'clsx';

type ButtonProps = React.ComponentProps<'button'> & {
  asChild?: boolean;
  variant?: 'default' | 'outline' | 'ghost' | 'destructive';
};
export function Button({ className, variant='default', asChild=false, style, children, ...props }: ButtonProps){
  const Component = asChild ? Slot : 'button';
  return <Component className={clsx('inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 disabled:opacity-50',
    variant==='default' && 'ui-button-primary',
    variant==='outline' && 'ui-button-outline',
    variant==='ghost' && 'ui-button-ghost',
    variant==='destructive' && 'ui-button-danger', className)} style={style} {...props}>{children}</Component>;
}
