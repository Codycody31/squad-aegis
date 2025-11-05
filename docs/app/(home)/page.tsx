import Link from 'next/link';
import Image from 'next/image';
import { 
  Server, 
  Shield, 
  Zap, 
  FileText, 
  Code, 
  Plug, 
  BarChart3,
  Users,
  ArrowRight,
  Terminal,
  Database,
  Activity,
  Heart,
  Github,
  Rocket,
  Settings,
  Monitor,
  Scale,
  Map,
  Clock,
  Ban,
  Ticket,
  History,
  FileUp
} from 'lucide-react';
import { cva } from 'class-variance-authority';
import { cn } from '@/lib/cn';

const buttonVariants = cva(
  'inline-flex items-center gap-2 px-5 py-3 rounded-full font-medium tracking-tight transition-colors',
  {
    variants: {
      variant: {
        primary: 'bg-fd-primary text-fd-primary-foreground hover:bg-fd-primary/80',
        secondary: 'border bg-fd-secondary text-fd-secondary-foreground hover:bg-fd-accent',
      },
    },
    defaultVariants: {
      variant: 'primary',
    },
  },
);

const cardVariants = cva('rounded-2xl text-sm p-6 bg-origin-border shadow-lg', {
  variants: {
    variant: {
      brand: 'bg-fd-primary text-fd-primary-foreground',
      secondary: 'bg-fd-secondary text-fd-secondary-foreground',
      default: 'border bg-fd-card',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export default function HomePage() {
  return (
    <main className="text-landing-foreground dark:text-landing-foreground-dark">
      {/* Hero Section with Background */}
      <div className="grid mx-auto w-full max-w-[1400px] relative">
        <div className="overflow-hidden col-start-1 row-start-1 absolute inset-0">
          <Image
            src="/assets/screenshots/server-teams-and-squads-mockup.png"
            alt="Squad Aegis Dashboard"
            fill
            className="object-cover"
            priority
            sizes="(max-width: 1400px) 100vw, 1400px"
          />
          <div className="absolute inset-0 bg-gradient-to-b from-background/80 via-background/60 to-background/80" />
          <div className="absolute inset-0 bg-gradient-to-t from-background via-transparent to-transparent" />
        </div>
        
        <div className="mt-auto text-landing-foreground mb-[max(100px,min(9vw,150px))] p-6 col-start-1 row-start-1 relative z-10 md:p-12">
          <div className="max-w-4xl">
            <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-fd-card border border-border text-sm font-medium mb-6">
              <Shield className="w-4 h-4 text-fd-primary" />
              <span>Comprehensive Squad Server Control Panel</span>
            </div>
            
            <h1 className="text-4xl mt-12 mb-6 leading-tighter font-bold md:text-5xl lg:text-6xl">
              Squad Aegis
              <br />
              <span className="text-fd-primary">Your Shield</span>
            </h1>
            
            <p className="text-xl text-muted-foreground mb-8 leading-relaxed max-w-2xl">
              A comprehensive control panel designed for efficient administration of Squad game servers. 
              Manage multiple servers through an intuitive web interface.
            </p>
            
            <div className="flex flex-col items-start gap-4 md:flex-row md:items-center">
              <Link href="/docs/installation" className={cn(buttonVariants())}>
                Getting Started
                <ArrowRight className="w-4 h-4" />
              </Link>
              <Link href="/docs" className={cn(buttonVariants({ variant: 'secondary' }))}>
                <FileText className="w-4 h-4" />
                Documentation
              </Link>
              <p className="text-sm text-muted-foreground">the Squad server management tool you love.</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 gap-8 px-6 pb-6 mx-auto w-full max-w-[1400px] md:px-12 md:pb-12 lg:grid-cols-2">
        {/* Intro Paragraph */}
        <p className="text-2xl tracking-tight leading-snug font-light col-span-full md:text-4xl">
          Squad Aegis is a <span className="text-fd-primary font-medium">comprehensive</span>{' '}
          control panel for{' '}
          <span className="text-fd-primary font-medium">Squad</span> game servers,
          designed by{' '}
          <span className="text-fd-primary font-medium">the community</span>. Bringing
          powerful features for server administration,
          to fit your needs, works seamlessly with Docker, supports multiple servers - everything you need.
        </p>

        {/* Quick Start Section */}
        <div className="p-8 bg-gradient-to-b from-fd-primary/10 rounded-xl col-span-full">
          <h2 className="text-4xl text-center font-bold mb-6 text-fd-primary">
            Get Started in Minutes
          </h2>
          <div className="bg-fd-card rounded-lg p-4 border border-border font-mono text-sm max-w-2xl mx-auto">
            <div className="flex items-center gap-2 mb-2">
              <Terminal className="w-4 h-4 text-fd-primary" />
              <span className="text-muted-foreground">docker-compose up -d</span>
            </div>
            <div className="text-xs text-muted-foreground">
              That's it! Squad Aegis will be running and ready to configure.
            </div>
          </div>
        </div>

        {/* Features Showcase */}
        <FeatureShowcase />
        
        {/* Screenshots Gallery */}
        <ScreenshotGallery />
        
        {/* Technology Stack */}
        <TechnologyStack />
        
        {/* Licensing */}
        <Licensing />
        
        {/* Open Source */}
        <OpenSource />
        
        {/* Roadmap */}
        <Roadmap />
        
        {/* Footer CTA */}
        <FooterCTA />
      </div>
    </main>
  );
}

function FeatureShowcase() {
  const features = [
    {
      icon: Server,
      title: 'Multi-Server Management',
      description: 'Supervise and control multiple Squad servers from a unified dashboard',
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-500/10',
    },
    {
      icon: Shield,
      title: 'Role-Based Access Control',
      description: 'Granular permission system ensuring secure administrative operations',
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-500/10',
    },
    {
      icon: Zap,
      title: 'Web-Based RCON Interface',
      description: 'Execute server commands through an intuitive graphical interface',
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-500/10',
    },
    {
      icon: FileText,
      title: 'Comprehensive Audit Logging',
      description: 'Track all administrative actions for security and accountability',
      color: 'text-purple-600 dark:text-purple-400',
      bgColor: 'bg-purple-500/10',
    },
    {
      icon: Plug,
      title: 'Plugin Architecture',
      description: 'Built-in Go plugins based on SquadJS equivalents',
      color: 'text-pink-600 dark:text-pink-400',
      bgColor: 'bg-pink-500/10',
    },
    {
      icon: BarChart3,
      title: 'Analytics & Monitoring',
      description: 'Monitor server performance and player statistics in real-time',
      color: 'text-orange-600 dark:text-orange-400',
      bgColor: 'bg-orange-500/10',
    },
  ];

  return (
    <>
      <div className={cn(cardVariants({ className: 'col-span-full' }))}>
        <h3 className="text-3xl font-bold mb-6">Powerful Features</h3>
        <p className="mb-8 text-muted-foreground">
          Everything you need to manage your Squad servers efficiently, all in one place.
        </p>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {features.map((feature, index) => {
            const Icon = feature.icon;
            return (
              <div
                key={index}
                className="group p-4 rounded-lg border border-border hover:border-fd-primary/50 transition-all"
              >
                <div className={`w-10 h-10 rounded-lg ${feature.bgColor} ${feature.color} flex items-center justify-center mb-3 group-hover:scale-110 transition-transform`}>
                  <Icon className="w-5 h-5" />
                </div>
                <h4 className="font-semibold mb-2">{feature.title}</h4>
                <p className="text-xs text-muted-foreground">{feature.description}</p>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}

function ScreenshotGallery() {
  const screenshots = [
    {
      src: '/assets/screenshots/server-teams-and-squads.png',
      alt: 'Server Teams and Squads',
      title: 'Server Teams and Squads',
      description: 'Manage teams and squads for your server',
    },
    {
      src: '/assets/screenshots/server-dashboard.png',
      alt: 'Server Dashboard',
      title: 'Server Management',
      description: 'Complete control over your Squad servers',
    },
    {
      src: '/assets/screenshots/server-plugins.png',
      alt: 'Plugin Management',
      title: 'Plugin System',
      description: 'Extend functionality with powerful plugins',
    },
  ];

  return (
    <>
      <div className={cn(cardVariants({ variant: 'brand', className: 'p-0 row-span-2' }))}>
        <div className="relative h-full min-h-[400px] rounded-2xl overflow-hidden">
          <Image
            src={screenshots[0].src}
            alt={screenshots[0].alt}
            fill
            className="object-cover"
            priority
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/60 to-transparent" />
          <div className="absolute bottom-0 left-0 right-0 p-6 text-white">
            <h3 className="text-xl font-bold mb-2">{screenshots[0].title}</h3>
            <p className="text-sm opacity-90">{screenshots[0].description}</p>
          </div>
        </div>
      </div>
      
      <div className={cn(cardVariants())}>
        <h3 className="text-2xl font-bold mb-4">Beautiful Interface</h3>
        <p className="text-muted-foreground mb-6">
          Squad Aegis offers a modern, intuitive interface designed for efficiency and ease of use.
        </p>
        <Link href="/docs/screenshots" className={cn(buttonVariants({ variant: 'secondary' }))}>
          View All Screenshots
          <ArrowRight className="w-4 h-4" />
        </Link>
      </div>
      
      <div className={cn(cardVariants({ className: 'flex flex-col' }))}>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div className="relative aspect-video rounded-lg overflow-hidden border border-border">
            <Image
              src={screenshots[1].src}
              alt={screenshots[1].alt}
              fill
              className="object-cover"
            />
          </div>
          <div className="relative aspect-video rounded-lg overflow-hidden border border-border">
            <Image
              src={screenshots[2].src}
              alt={screenshots[2].alt}
              fill
              className="object-cover"
            />
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          {screenshots[1].title} • {screenshots[2].title}
        </p>
      </div>
    </>
  );
}

function TechnologyStack() {
  return (
    <>
      <h2 className="text-3xl lg:text-4xl font-bold text-fd-primary text-center mb-8 col-span-full">
        Modern Technology Stack
      </h2>
      
      <div className={cn(cardVariants({ className: 'col-span-full' }))}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
          {[
            { name: 'Go', desc: 'Backend' },
            { name: 'Vue.js', desc: 'Frontend' },
            { name: 'PostgreSQL & Clickhouse', desc: 'Database' },
            { name: 'Docker', desc: 'Deployment' },
          ].map((tech) => (
            <div key={tech.name} className="text-center">
              <div className="text-2xl font-bold mb-2">{tech.name}</div>
              <div className="text-xs text-muted-foreground">{tech.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

function Licensing() {
  return (
    <>
      <h2 className="text-3xl lg:text-4xl font-bold text-fd-primary text-center mb-8 col-span-full mt-8">
        Licensing & Usage Rights
      </h2>
      
      <div className={cn(cardVariants({ className: 'flex flex-col col-span-full' }))}>
        <Scale className="w-8 h-8 mb-4 text-fd-primary" />
        <h3 className="text-2xl font-bold mb-6">Elastic License 2.0</h3>
        <p className="mb-6 text-muted-foreground">
          Squad Aegis is licensed under the Elastic License 2.0 (ELv2), a source-available license 
          that provides flexibility while protecting the project. Here's what this means for you:
        </p>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
          <div className="p-4 rounded-lg border border-border bg-fd-muted/30">
            <h4 className="font-semibold mb-2 flex items-center gap-2">
              <span className="text-green-600 dark:text-green-400">✓</span>
              What You Can Do
            </h4>
            <ul className="text-sm text-muted-foreground space-y-1 list-disc list-inside">
              <li>Use Squad Aegis for any purpose</li>
              <li>Modify the source code</li>
              <li>Contribute improvements</li>
              <li>Share the code with others</li>
              <li>Run it on your own infrastructure</li>
            </ul>
          </div>
          
          <div className="p-4 rounded-lg border border-border bg-fd-muted/30">
            <h4 className="font-semibold mb-2 flex items-center gap-2">
              <span className="text-red-600 dark:text-red-400">✗</span>
              What You Cannot Do
            </h4>
            <ul className="text-sm text-muted-foreground space-y-1 list-disc list-inside">
              <li>Provide Squad Aegis as a hosted service</li>
              <li>Circumvent license key functionality</li>
              <li>Remove or alter license notices</li>
              <li>Create competing hosted services</li>
            </ul>
          </div>
        </div>
        
        <div className="p-4 rounded-lg border border-fd-primary/20 bg-fd-primary/5">
          <p className="text-sm text-muted-foreground mb-2">
            <strong className="text-foreground">Key Point:</strong> You can use, modify, and deploy 
            Squad Aegis freely for your own Squad server management needs. The license only restricts 
            offering it as a commercial hosted service.
          </p>
        </div>
        
        <div className="mt-6 flex flex-row items-center gap-2">
          <a
            href="https://github.com/Codycody31/squad-aegis/blob/master/LICENSE"
            rel="noreferrer noopener"
            target="_blank"
            className={cn(buttonVariants({ variant: 'secondary' }))}
          >
            <FileText className="w-4 h-4" />
            Read Full License
          </a>
          <a
            href="https://www.elastic.co/licensing/elastic-license"
            rel="noreferrer noopener"
            target="_blank"
            className={cn(buttonVariants({ variant: 'secondary' }))}
          >
            Learn About ELv2
          </a>
        </div>
      </div>
    </>
  );
}

function OpenSource() {
  return (
    <>
      <h2 className="text-3xl lg:text-4xl font-bold text-fd-primary text-center mb-8 col-span-full mt-8">
        Source Available & Community Driven
      </h2>
      
      <div className={cn(cardVariants({ className: 'flex flex-col' }))}>
        <Heart fill="currentColor" className="text-pink-500 mb-4 w-8 h-8" />
        <h3 className="text-2xl font-bold mb-6">Made Possible by You</h3>
        <p className="mb-8 text-muted-foreground">
          Squad Aegis is source-available and powered by the community. 
          Contributions, feedback, and support make this project possible.
        </p>
        <div className="mb-8 flex flex-row items-center gap-2">
          <a
            href="https://github.com/Codycody31/squad-aegis"
            rel="noreferrer noopener"
            target="_blank"
            className={cn(buttonVariants())}
          >
            <Github className="w-4 h-4" />
            View on GitHub
          </a>
          <Link href="/docs/developers/overview" className={cn(buttonVariants({ variant: 'secondary' }))}>
            Developer Docs
          </Link>
        </div>
      </div>
      
      <ul className={cn(cardVariants({ className: 'flex flex-col gap-6' }))}>
        <li>
          <span className="flex flex-row items-center gap-2 font-medium">
            <Rocket className="w-5 h-5 text-fd-primary" />
            Actively Maintained
          </span>
          <span className="mt-2 text-sm text-muted-foreground block">
            Regular updates and improvements, open for contributions.
          </span>
        </li>
        <li>
          <span className="flex flex-row items-center gap-2 font-medium">
            <Github className="w-5 h-5 text-fd-primary" />
            Elastic License 2.0
          </span>
          <span className="mt-2 text-sm text-muted-foreground block">
            Source-available, available on GitHub under Elastic License 2.0.
          </span>
        </li>
        <li>
          <span className="flex flex-row items-center gap-2 font-medium">
            <Activity className="w-5 h-5 text-fd-primary" />
            Community Driven
          </span>
          <span className="mt-2 text-sm text-muted-foreground block">
            Built by the community, for the community.
          </span>
        </li>
        <li className="flex flex-row flex-wrap gap-2 mt-auto">
          <Link href="/docs/installation" className={cn(buttonVariants())}>
            Get Started
          </Link>
          <a
            href="https://github.com/Codycody31/squad-aegis"
            rel="noreferrer noopener"
            target="_blank"
            className={cn(buttonVariants({ variant: 'secondary' }))}
          >
            <Github className="w-4 h-4" />
            GitHub
          </a>
        </li>
      </ul>
    </>
  );
}

function Roadmap() {
  const roadmapItems = [
    {
      icon: Ban,
      title: 'IP Banlists',
      description: 'Enhanced IP-based banning system for improved server security',
      color: 'text-red-600 dark:text-red-400',
      bgColor: 'bg-red-500/10',
    },
    {
      icon: Ticket,
      title: 'Ticket System & File Uploads',
      description: 'Upload files to track proof of rule violations for permanent bans',
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-500/10',
    },
    {
      icon: FileUp,
      title: 'Smart Ban Reasons',
      description: 'Predetermined ban reason suggestions based on player history and rules',
      color: 'text-purple-600 dark:text-purple-400',
      bgColor: 'bg-purple-500/10',
    },
    {
      icon: Shield,
      title: 'Blacklists',
      description: 'Comprehensive blacklist management system',
      color: 'text-orange-600 dark:text-orange-400',
      bgColor: 'bg-orange-500/10',
    },
    {
      icon: History,
      title: 'Player History & Admin Notes',
      description: 'Complete history of warnings, kicks, bans, TKs, kills, deaths, and admin notes',
      color: 'text-green-600 dark:text-green-400',
      bgColor: 'bg-green-500/10',
    },
    {
      icon: Clock,
      title: 'Data Retention Policies',
      description: 'Configurable retention policies (e.g., chat history 30 days unless linked to rule breakage)',
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-500/10',
    },
    {
      icon: Map,
      title: 'Enhanced Map & Round History',
      description: 'Cleaner map history and round match history pages',
      color: 'text-pink-600 dark:text-pink-400',
      bgColor: 'bg-pink-500/10',
    },
  ];

  return (
    <>
      <h2 className="text-3xl lg:text-4xl font-bold text-fd-primary text-center mb-8 col-span-full mt-8">
        Roadmap
      </h2>
      
      <div className={cn(cardVariants({ className: 'col-span-full' }))}>
        <p className="text-muted-foreground mb-8 text-center">
          Planned features and improvements coming to Squad Aegis. These items are not tracked chronologically.
        </p>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {roadmapItems.map((item, index) => {
            const Icon = item.icon;
            return (
              <div
                key={index}
                className="group p-5 rounded-lg border border-border hover:border-fd-primary/50 transition-all bg-fd-card/50"
              >
                <div className={`w-10 h-10 rounded-lg ${item.bgColor} ${item.color} flex items-center justify-center mb-4 group-hover:scale-110 transition-transform`}>
                  <Icon className="w-5 h-5" />
                </div>
                <h4 className="font-semibold mb-2 text-base">{item.title}</h4>
                <p className="text-sm text-muted-foreground leading-relaxed">{item.description}</p>
              </div>
            );
          })}
        </div>
        
        <div className="mt-8 p-4 rounded-lg border border-fd-primary/20 bg-fd-primary/5 text-center">
          <p className="text-sm text-muted-foreground">
            Have suggestions or want to contribute?{' '}
            <a
              href="https://github.com/Codycody31/squad-aegis"
              rel="noreferrer noopener"
              target="_blank"
              className="text-fd-primary hover:underline font-medium"
            >
              Check out our GitHub
            </a>
          </p>
        </div>
      </div>
    </>
  );
}

function FooterCTA() {
  return (
    <footer className="flex flex-col justify-center items-center bg-fd-secondary py-12 text-fd-secondary-foreground rounded-2xl col-span-full">
      <p className="mb-1 text-3xl font-semibold">Squad Aegis</p>
      <p className="text-sm mb-6 text-muted-foreground">
        Comprehensive Squad Server Control Panel
      </p>
      <div className="flex gap-4">
        <Link href="/docs/installation" className={cn(buttonVariants())}>
          <Rocket className="w-4 h-4" />
          Start Installation
        </Link>
        <Link href="/docs" className={cn(buttonVariants({ variant: 'secondary' }))}>
          <FileText className="w-4 h-4" />
          Documentation
        </Link>
      </div>
    </footer>
  );
}
