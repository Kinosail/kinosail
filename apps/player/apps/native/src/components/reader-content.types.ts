export type ReaderContentProps = {
  server: string;
  authorization: string;
  id: string;
  path: string;
  type: 'epub' | 'pdf' | 'comic';
  fontSize: number;
  theme: 'light' | 'dark' | 'sepia';
  offset?: number;
  onPosition?(event: { nativeEvent: { path: string; offset: number } }): void;
  onError(): void;
};
