import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { BookIcon } from 'lucide-react';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: 'Squad Aegis',
    },
    githubUrl: 'https://github.com/Codycody31/squad-aegis',
  };
}
