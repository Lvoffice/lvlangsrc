using System;
using System.Text;

namespace Lvl
{
    /// <summary>
    /// 终端输出适配器 - 自动检测并转换编码
    /// 不改变终端编码，只适配程序输出
    /// </summary>
    public static class Terminal
    {
        private static readonly Encoding Utf8 = Encoding.UTF8;
        private static readonly Encoding Gbk = Encoding.GetEncoding(936);
        private static readonly Encoding TerminalEncoding = Console.OutputEncoding;
        
        /// <summary>
        /// 检测是否需要 GBK 转换
        /// </summary>
        public static bool NeedConvert => TerminalEncoding.CodePage == 936;
        
        /// <summary>
        /// 将 UTF-8 字符串转换为终端编码
        /// </summary>
        public static string Convert(string s)
        {
            if (string.IsNullOrEmpty(s)) return s;
            
            if (NeedConvert)
            {
                byte[] gbkBytes = Encoding.Convert(Utf8, Gbk, Utf8.GetBytes(s));
                return Gbk.GetString(gbkBytes);
            }
            
            return s;
        }
        
        /// <summary>
        /// 输出到标准输出
        /// </summary>
        public static void Print(string s)
        {
            Console.Write(Convert(s));
        }
        
        /// <summary>
        /// 输出到标准输出（带换行）
        /// </summary>
        public static void PrintLine(string s)
        {
            Console.WriteLine(Convert(s));
        }
        
        /// <summary>
        /// 格式化输出
        /// </summary>
        public static void PrintFormat(string format, params object[] args)
        {
            Console.Write(Convert(string.Format(format, args)));
        }
        
        /// <summary>
        /// 输出到标准错误
        /// </summary>
        public static void Error(string s)
        {
            var originalColor = Console.ForegroundColor;
            Console.ForegroundColor = ConsoleColor.Red;
            Console.Error.Write(Convert(s));
            Console.ForegroundColor = originalColor;
        }
        
        public static void ErrorLine(string s)
        {
            Error(s + Environment.NewLine);
        }
    }
}
