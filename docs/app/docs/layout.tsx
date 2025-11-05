import { source } from '@/lib/source';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions } from '@/lib/layout.shared';

export default function Layout({ children }: LayoutProps<'/docs'>) {
  return (
    <DocsLayout tree={source.pageTree} {...baseOptions()} githubUrl='https://github.com/Codycody31/squad-aegis'>
      {children}
    </DocsLayout>
  );
}
