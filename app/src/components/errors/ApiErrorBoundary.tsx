import React, { Component, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { RefreshCw, Home } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onNavigateHome?: () => void; // Add callback for navigation
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ApiErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('API Error Boundary caught an error:', error, errorInfo);

    // Could send to error reporting service
    // this.reportError(error, errorInfo);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: undefined });
  };

  handleGoHome = () => {
    window.location.href = "/setup";
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="fixed inset-0 flex items-center justify-center bg-background p-4">
          <Card className="w-full max-w-md p-8 bg-[var(--variable-collection-surface)] border-[var(--shadcn-ui-app-border)] shadow-2xl">
            <div className="flex flex-col items-center text-center space-y-6">
              <div className="space-y-2">
                <h2 className="text-2xl font-semibold text-[var(--tailwind-colors-slate-50)]">
                  Something went wrong
                </h2>
                <p className="text-[var(--tailwind-colors-slate-400)] text-base">
                  An unexpected error occurred. Please try again or return to the home page.
                </p>
              </div>

              <div className="flex flex-col sm:flex-row gap-3 w-full">
                <Button
                  onClick={this.handleRetry}
                  variant="outline"
                  className="flex items-center gap-2 flex-1 border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-400)] hover:bg-[var(--tailwind-colors-slate-800)]"
                >
                  <RefreshCw className="w-4 h-4" />
                  Try Again
                </Button>
                <Button
                  onClick={this.handleGoHome}
                  className="flex items-center gap-2 flex-1 bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] hover:bg-[var(--tailwind-colors-rdns-800)]"
                >
                  <Home className="w-4 h-4" />
                  Home
                </Button>
              </div>
            </div>
          </Card>
        </div>
      );
    }

    return this.props.children;
  }
}