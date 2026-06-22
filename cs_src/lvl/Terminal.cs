using System;
using System.Runtime.InteropServices;
using System.Text;

namespace LvlTerminal
{
    public static class Term
    {
        private static readonly Encoding Utf8 = new UTF8Encoding(false);
        private static readonly Encoding Gbk;
        private static readonly int ConsoleCodePage;
        
        [DllImport("kernel32.dll")]
        private static extern int GetConsoleOutputCP();
        
        [DllImport("kernel32.dll")]
        private static extern bool WriteFile(IntPtr hFile, byte[] lpBuffer, uint nNumberOfBytesToWrite, out uint lpNumberOfBytesWritten, IntPtr lpOverlapped);
        
        [DllImport("kernel32.dll")]
        private static extern IntPtr GetStdHandle(int nStdHandle);
        
        private static readonly IntPtr stdoutHandle;
        
        static Term()
        {
            Encoding.RegisterProvider(CodePagesEncodingProvider.Instance);
            Gbk = Encoding.GetEncoding(936);
            ConsoleCodePage = GetConsoleOutputCP();
            stdoutHandle = GetStdHandle(-11);
        }
        
        /// <summary>
        /// 直接写入字节到标准输出（绕过 Console 编码处理）
        /// </summary>
        private static void WriteBytes(byte[] bytes)
        {
            if (bytes == null || bytes.Length == 0) return;
            WriteFile(stdoutHandle, bytes, (uint)bytes.Length, out _, IntPtr.Zero);
        }
        
        /// <summary>
        /// 输出字符串（自动适配终端编码）
        /// </summary>
        private static void WriteString(string s)
        {
            if (string.IsNullOrEmpty(s)) return;
            
            byte[] bytes;
            if (ConsoleCodePage == 936)
            {
                // GBK 终端：转 GBK
                byte[] utf8Bytes = Utf8.GetBytes(s);
                bytes = Encoding.Convert(Utf8, Gbk, utf8Bytes);
            }
            else
            {
                // UTF-8：直接写
                bytes = Utf8.GetBytes(s);
            }
            WriteBytes(bytes);
        }
        
        /// <summary>
        /// 从 ANSI 指针读取字符串（Go DLL 用 fmt.Println 输出，编码取决于系统代码页）
        /// 在 GBK 代码页下是 GBK 编码，在 UTF-8 代码页下是 UTF-8 编码
        /// </summary>
        public static string FromAnsi(IntPtr ptr)
        {
            if (ptr == IntPtr.Zero) return "";
            
            int len = 0;
            while (Marshal.ReadByte(ptr, len) != 0) len++;
            
            if (len == 0) return "";
            
            byte[] bytes = new byte[len];
            Marshal.Copy(ptr, bytes, 0, len);
            
            // 根据控制台代码页决定如何解码
            if (ConsoleCodePage == 936)
            {
                // GBK 代码页：DLL 输出的是 GBK 编码
                try { return Gbk.GetString(bytes); }
                catch { return Encoding.ASCII.GetString(bytes); }
            }
            else
            {
                // UTF-8 代码页：DLL 输出的是 UTF-8 编码
                try { return Utf8.GetString(bytes); }
                catch { return Encoding.ASCII.GetString(bytes); }
            }
        }
        
        /// <summary>
        /// 输出字符串
        /// </summary>
        public static void Print(string s)
        {
            WriteString(s);
        }
        
        /// <summary>
        /// 输出字符串并换行
        /// </summary>
        public static void PrintLine(string s)
        {
            WriteString(s);
            WriteBytes(Utf8.GetBytes("\r\n"));
        }
        
        /// <summary>
        /// 输出错误信息
        /// </summary>
        public static void ErrorLine(string s)
        {
            WriteString("[错误] ");
            WriteString(s);
            WriteBytes(Utf8.GetBytes("\r\n"));
        }
    }
}
