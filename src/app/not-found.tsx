import Link from "next/link";

export default function NotFound() {
  return (
    <main id="main-content" className="flex min-h-screen flex-col items-center justify-center p-8">
      <div className="max-w-md text-center">
        <h1 className="text-6xl font-bold text-[var(--color-accent)] mb-4">404</h1>
        <h2 className="text-xl font-semibold text-[var(--color-text-primary)] mb-2">
          Page not found
        </h2>
        <p className="text-[var(--color-text-secondary)] mb-8">
          The page you&apos;re looking for doesn&apos;t exist or has been moved.
        </p>
        <Link
          href="/"
          className="rounded-full bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors"
        >
          Back to Toqui
        </Link>
      </div>
    </main>
  );
}
