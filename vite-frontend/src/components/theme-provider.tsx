import React from 'react';

interface ThemeProviderProps {
  children: React.ReactNode;
}

// 统一使用白底黑字，保留组件以避免改变 Provider 结构。
export const ThemeProvider: React.FC<ThemeProviderProps> = ({ children }) => <>{children}</>;
