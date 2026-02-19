const MAP_URL = "https://map.sentryatlas.com";
const GITHUB_URL = "https://github.com/Kohantic/SentryAtlas";

const DATA_SOURCES = [
  {
    name: "USGS",
    description: "Earthquakes worldwide, updated in near real-time",
    url: "https://earthquake.usgs.gov",
  },
  {
    name: "NASA EONET",
    description: "Wildfires, volcanoes, storms, and icebergs tracked by NASA",
    url: "https://eonet.gsfc.nasa.gov",
  },
  {
    name: "NOAA / NWS",
    description: "Floods, tornados, hurricanes, and winter storm alerts across the US",
    url: "https://www.weather.gov",
  },
  {
    name: "GDACS",
    description: "Global disaster alerts for cyclones, droughts, floods, and more",
    url: "https://www.gdacs.org",
  },
];

const EVENT_TYPES = [
  "Earthquakes",
  "Wildfires",
  "Volcanoes",
  "Storms",
  "Floods",
  "Cyclones",
  "Tornados",
  "Hurricanes",
  "Winter Storms",
  "Tsunamis",
  "Droughts",
  "Icebergs",
  "Landslides",
];

function GlobeIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20" />
      <path d="M2 12h20" />
    </svg>
  );
}

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="currentColor"
    >
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  );
}

function ArrowRightIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M5 12h14" />
      <path d="m12 5 7 7-7 7" />
    </svg>
  );
}

function LayersIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z" />
      <path d="m2 12 8.58 3.91a2 2 0 0 0 1.66 0L20.76 12" />
      <path d="m2 17 8.58 3.91a2 2 0 0 0 1.66 0L20.76 17" />
    </svg>
  );
}

function CodeIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polyline points="16 18 22 12 16 6" />
      <polyline points="8 6 2 12 8 18" />
    </svg>
  );
}

function ShieldIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z" />
    </svg>
  );
}

export default function Home() {
  return (
    <div className="min-h-screen">
      {/* Navigation */}
      <nav className="fixed top-0 left-0 right-0 z-50 bg-brand-neutral/80 backdrop-blur-md border-b border-brand-black/5">
        <div className="max-w-5xl mx-auto px-6 h-16 flex items-center justify-between">
          <span className="text-lg font-bold tracking-tight text-brand-black">
            Sentry<span className="text-brand-accent">Atlas</span>
          </span>
          <div className="flex items-center gap-4">
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="text-brand-black/50 hover:text-brand-black transition-colors"
              aria-label="GitHub"
            >
              <GitHubIcon className="w-5 h-5" />
            </a>
            <a
              href={MAP_URL}
              className="text-sm font-medium bg-brand-accent text-white px-4 py-2 rounded-full hover:bg-brand-accent/90 transition-colors"
            >
              Open Map
            </a>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="pt-40 pb-24 px-6">
        <div className="max-w-3xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 text-xs font-medium text-brand-black/50 bg-brand-black/5 rounded-full px-3 py-1.5 mb-8">
            <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
            Open source &amp; free forever
          </div>
          <h1 className="text-5xl sm:text-6xl font-bold tracking-tight text-brand-black leading-[1.1] mb-6">
            Real-time disaster{" "}
            <span className="text-brand-accent">monitoring</span>,{" "}
            <br className="hidden sm:block" />
            open and free
          </h1>
          <p className="text-lg sm:text-xl text-brand-black/60 max-w-2xl mx-auto mb-10 leading-relaxed">
            SentryAtlas aggregates live disaster data from four trusted public
            sources onto a single interactive map. Earthquakes, wildfires,
            floods, storms — see it all at a glance.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <a
              href={MAP_URL}
              className="group flex items-center gap-2 text-base font-semibold bg-brand-accent text-white px-7 py-3.5 rounded-full hover:bg-brand-accent/90 transition-colors shadow-lg shadow-brand-accent/25"
            >
              Explore the Map
              <ArrowRightIcon className="w-4 h-4 group-hover:translate-x-0.5 transition-transform" />
            </a>
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-base font-semibold text-brand-black border-2 border-brand-black/15 px-7 py-3.5 rounded-full hover:border-brand-black/30 transition-colors"
            >
              <GitHubIcon className="w-5 h-5" />
              View on GitHub
            </a>
          </div>
        </div>
      </section>

      {/* Event Types Ticker */}
      <section className="py-8 border-y border-brand-black/5">
        <div className="max-w-5xl mx-auto px-6">
          <div className="flex flex-wrap items-center justify-center gap-3">
            {EVENT_TYPES.map((type) => (
              <span
                key={type}
                className="text-xs font-medium text-brand-black/40 bg-brand-black/[0.03] rounded-full px-3 py-1.5"
              >
                {type}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* What Is SentryAtlas */}
      <section className="py-24 px-6">
        <div className="max-w-5xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-brand-black mb-4">
              One map. Every disaster. In real time.
            </h2>
            <p className="text-brand-black/50 max-w-2xl mx-auto text-lg">
              SentryAtlas continuously pulls data from government and
              scientific agencies around the world, normalizes it, and plots it
              on an interactive heatmap you can filter, zoom, and explore.
            </p>
          </div>

          <div className="grid sm:grid-cols-3 gap-8">
            <div className="bg-white rounded-2xl p-8 shadow-sm border border-brand-black/5">
              <div className="w-12 h-12 bg-brand-accent/10 rounded-xl flex items-center justify-center mb-5">
                <GlobeIcon className="w-6 h-6 text-brand-accent" />
              </div>
              <h3 className="font-semibold text-brand-black mb-2">Global Coverage</h3>
              <p className="text-sm text-brand-black/50 leading-relaxed">
                Aggregates data from 4 independent sources covering every
                continent and ocean. Earthquakes in Japan, wildfires in
                California, floods in Europe — all in one place.
              </p>
            </div>
            <div className="bg-white rounded-2xl p-8 shadow-sm border border-brand-black/5">
              <div className="w-12 h-12 bg-brand-accent/10 rounded-xl flex items-center justify-center mb-5">
                <LayersIcon className="w-6 h-6 text-brand-accent" />
              </div>
              <h3 className="font-semibold text-brand-black mb-2">13 Event Types</h3>
              <p className="text-sm text-brand-black/50 leading-relaxed">
                From earthquakes and wildfires to hurricanes and icebergs.
                Filter by type, time range, and viewport to see exactly what
                matters to you.
              </p>
            </div>
            <div className="bg-white rounded-2xl p-8 shadow-sm border border-brand-black/5">
              <div className="w-12 h-12 bg-brand-accent/10 rounded-xl flex items-center justify-center mb-5">
                <ShieldIcon className="w-6 h-6 text-brand-accent" />
              </div>
              <h3 className="font-semibold text-brand-black mb-2">Trusted Data</h3>
              <p className="text-sm text-brand-black/50 leading-relaxed">
                Every event comes from a verified public agency. No
                user-submitted data, no social media scraping — just
                authoritative government and scientific sources.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Data Sources */}
      <section className="py-24 px-6 bg-brand-black">
        <div className="max-w-5xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-brand-neutral mb-4">
              Powered by public data
            </h2>
            <p className="text-brand-neutral/50 max-w-xl mx-auto text-lg">
              SentryAtlas pulls from four authoritative, freely available data
              sources — updated continuously.
            </p>
          </div>

          <div className="grid sm:grid-cols-2 gap-6">
            {DATA_SOURCES.map((source) => (
              <a
                key={source.name}
                href={source.url}
                target="_blank"
                rel="noopener noreferrer"
                className="group bg-white/[0.05] hover:bg-white/[0.08] border border-white/[0.08] rounded-2xl p-6 transition-colors"
              >
                <h3 className="text-lg font-semibold text-brand-neutral mb-1.5 group-hover:text-brand-accent transition-colors">
                  {source.name}
                </h3>
                <p className="text-sm text-brand-neutral/40 leading-relaxed">
                  {source.description}
                </p>
              </a>
            ))}
          </div>
        </div>
      </section>

      {/* Open Source */}
      <section className="py-24 px-6">
        <div className="max-w-3xl mx-auto text-center">
          <div className="w-14 h-14 bg-brand-accent/10 rounded-2xl flex items-center justify-center mx-auto mb-6">
            <CodeIcon className="w-7 h-7 text-brand-accent" />
          </div>
          <h2 className="text-3xl sm:text-4xl font-bold text-brand-black mb-4">
            Fully open source
          </h2>
          <p className="text-brand-black/50 max-w-xl mx-auto text-lg mb-4 leading-relaxed">
            SentryAtlas is built in the open with a Go backend, a Next.js
            frontend, and MapLibre GL for map rendering. No API keys required,
            no paywalls, no tracking.
          </p>
          <p className="text-brand-black/40 text-sm mb-10">
            Contributions, issues, and ideas are welcome.
          </p>
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 text-base font-semibold text-brand-black border-2 border-brand-black/15 px-7 py-3.5 rounded-full hover:border-brand-black/30 transition-colors"
          >
            <GitHubIcon className="w-5 h-5" />
            View on GitHub
          </a>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-10 px-6 border-t border-brand-black/5">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-4">
          <span className="text-sm text-brand-black/40">
            Made with ♥ by the{" "}
            <a href="https://kohantic.com" target="_blank" rel="noopener noreferrer" className="font-bold text-brand-accent hover:underline">KOHANTIC</a> team
          </span>
          <div className="flex items-center gap-6 text-sm text-brand-black/40">
            <a
              href={MAP_URL}
              className="hover:text-brand-black transition-colors"
            >
              Map
            </a>
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-brand-black transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
