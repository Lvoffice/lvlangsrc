using System;
using System.IO;
using System.Runtime.InteropServices;

namespace Lvlp
{
    class Program
    {
        private static IntPtr _parserDll = IntPtr.Zero;

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ParseFileDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ParseFileDebugDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate void FreeStringDelegate(IntPtr str);

        static int Main(string[] args)
        {
            try
            {
                string exePath = Environment.ProcessPath!;
                if (string.IsNullOrEmpty(exePath))
                {
                    LvlTerminal.Term.ErrorLine("获取程序路径失败");
                    return 1;
                }

                string exeDir = Path.GetDirectoryName(exePath)!;
                string projectDir = Directory.GetParent(exeDir)!.FullName;
                string parserDllPath = Path.Combine(projectDir, "libs", "parser.dll");

                _parserDll = LoadDll(parserDllPath);
                if (_parserDll == IntPtr.Zero)
                {
                    LvlTerminal.Term.ErrorLine("加载解析器失败: " + parserDllPath);
                    return 1;
                }

                bool debug = false;
                for (int i = 0; i < args.Length; i++)
                {
                    if (args[i] == "-d") debug = true;
                }

                // 找到第一个非 flag 参数作为文件名
                string filename = "";
                foreach (var arg in args)
                {
                    if (!arg.StartsWith("-"))
                    {
                        filename = arg;
                        break;
                    }
                }

                if (string.IsNullOrEmpty(filename))
                {
                    LvlTerminal.Term.PrintLine("用法: lvlp [-d] <文件.lvls>");
                    LvlTerminal.Term.PrintLine("  -d    调试模式");
                    LvlTerminal.Term.PrintLine("");
                    LvlTerminal.Term.PrintLine("示例:");
                    LvlTerminal.Term.PrintLine("  lvlp test.lvls        # 解析并输出AST");
                    LvlTerminal.Term.PrintLine("  lvlp -d test.lvls     # 调试模式解析");
                    LvlTerminal.Term.PrintLine("");
                    LvlTerminal.Term.PrintLine("DLL目录: " + Path.Combine(projectDir, "libs"));
                    return 1;
                }
                if (!Path.IsPathRooted(filename))
                    filename = Path.GetFullPath(filename);

                try
                {
                    Directory.SetCurrentDirectory(projectDir);
                }
                catch (Exception ex)
                {
                    LvlTerminal.Term.ErrorLine("切换目录失败: " + ex.Message);
                    return 1;
                }

                string result = CallParseFile(filename, debug);
                LvlTerminal.Term.PrintLine(result);

                return 0;
            }
            catch (Exception ex)
            {
                LvlTerminal.Term.ErrorLine("程序异常: " + ex.Message);
                return 1;
            }
            finally
            {
                if (_parserDll != IntPtr.Zero)
                {
                    FreeLibrary(_parserDll);
                    _parserDll = IntPtr.Zero;
                }
            }
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr LoadLibrary(string lpFileName);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool FreeLibrary(IntPtr hModule);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr GetProcAddress(IntPtr hModule, string lpProcName);

        private static IntPtr LoadDll(string dllPath)
        {
            IntPtr hModule = LoadLibrary(dllPath);
            if (hModule == IntPtr.Zero)
                throw new DllNotFoundException("无法加载 DLL: " + dllPath);
            return hModule;
        }

        private static string CallParseFile(string filename, bool debug)
        {
            IntPtr proc = GetProcAddress(_parserDll, debug ? "ParseFileDebug" : "ParseFile");
            if (proc == IntPtr.Zero)
                return "错误: 函数未找到";

            IntPtr result;
            if (debug)
            {
                var func = Marshal.GetDelegateForFunctionPointer<ParseFileDebugDelegate>(proc);
                result = func(filename);
            }
            else
            {
                var func = Marshal.GetDelegateForFunctionPointer<ParseFileDelegate>(proc);
                result = func(filename);
            }

            if (result == IntPtr.Zero)
                return "";

            string str = LvlTerminal.Term.FromAnsi(result);

            IntPtr freeProc = GetProcAddress(_parserDll, "FreeString");
            if (freeProc != IntPtr.Zero)
            {
                var freeFunc = Marshal.GetDelegateForFunctionPointer<FreeStringDelegate>(freeProc);
                freeFunc(result);
            }

            return str;
        }
    }
}
