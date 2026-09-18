declare module "@nimiq/identicons/dist/identicons.bundle.min.js" {
  export default class Identicons {
    static svg(text: string): Promise<string>;
    static toDataUrl(text: string): Promise<string>;
    static render(text: string, element: Element): Promise<void>;
    static image(text: string): Promise<HTMLImageElement>;
    static placeholder(color?: string, strokeWidth?: number): string;
    static placeholderToDataUrl(color?: string, strokeWidth?: number): string;
    static renderPlaceholder(
      element: Element,
      color?: string,
      strokeWidth?: number,
    ): void;
  }
}
