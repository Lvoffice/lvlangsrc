using System;
using System.IO;
using System.Runtime.InteropServices;

namespace Lvl
{
    class Program
    {
        // DLL 句柄
        private static IntPtr _parserDll = IntPtr.Zero;
        private static IntPtr _interpreterDll = IntPtr.Zero;

        // DLL 函数委托
        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ParseFileDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ParseFileDebugDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ExecuteFileDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate IntPtr ExecuteFileDebugDelegate([MarshalAs(UnmanagedType.LPStr)] string filename);

        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        private delegate void FreeStringDelegate(IntPtr str);

        static int Main(string[] args)
        {
            try
            {
                // 获取程序自身路径
                string exePath = Environment.ProcessPath!;
                if (string.IsNullOrEmpty(exePath))
                {
                    LvlTerminal.Term.ErrorLine("获取程序路径失败");
                    return 1;
                }

                string exeDir = Path.GetDirectoryName(exePath)!;
                string projectDir = Directory.GetParent(exeDir)!.FullName;

                // 指定 DLL 路径
                string parserDllPath = Path.Combine(projectDir, "libs", "parser.dll");
                string interpreterDllPath = Path.Combine(projectDir, "libs", "interpreter.dll");
                string cacheDir = Path.Combine(projectDir, "cache");

                // 加载 DLL
                _parserDll = LoadDll(parserDllPath);
                if (_parserDll == IntPtr.Zero)
                {
                    LvlTerminal.Term.ErrorLine($"加载解析器失败，请确保文件存在: {parserDllPath}");
                    return 1;
                }

                _interpreterDll = LoadDll(interpreterDllPath);
                if (_interpreterDll == IntPtr.Zero)
                {
                    LvlTerminal.Term.ErrorLine($"加载解释器失败，请确保文件存在: {interpreterDllPath}");
                    return 1;
                }

                // 解析命令行参数
                bool debug = false;
                bool clean = false;

                for (int i = 0; i < args.Length; i++)
                {
                    switch (args[i])
                    {
                        case "-d":
                            debug = true;
                            break;
                        case "-c":
                        case "-clean":
                            clean = true;
                            break;
                    }
                }

                // 处理清理命令
                if (clean)
                {
                    if (Directory.Exists(cacheDir))
                    {
                        try
                        {
                            Directory.Delete(cacheDir, true);
                            LvlTerminal.Term.PrintLine("缓存已清理");
                        }
                        catch (Exception ex)
                        {
                            LvlTerminal.Term.ErrorLine($"清理缓存失败: {ex.Message}");
                            return 1;
                        }
                    }
                    else
                    {
                        LvlTerminal.Term.PrintLine("缓存已清理");
                    }
                    return 0;
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

                // 检查文件参数
                if (string.IsNullOrEmpty(filename))
                {
                    LvlTerminal.Term.PrintLine("用法: lvl [-d] <文件.lvls>");
                    LvlTerminal.Term.PrintLine("       lvl -c            清理缓存");
                    LvlTerminal.Term.PrintLine("       lvl -clean        同上");
                    LvlTerminal.Term.PrintLine("  -d    调试模式");
                    LvlTerminal.Term.PrintLine("");
                    LvlTerminal.Term.Print("DLL目录: " + Path.Combine(projectDir, "libs") + "\n");
                    LvlTerminal.Term.Print("缓存目录: " + cacheDir + "\n");
                    return 1;
                }
                if (!Path.IsPathRooted(filename))
                {
                    filename = Path.GetFullPath(filename);
                }

                // 切换到项目根目录
                try
                {
                    Directory.SetCurrentDirectory(projectDir);
                }
                catch (Exception ex)
                {
                    LvlTerminal.Term.ErrorLine($"切换目录失败: {ex.Message}");
                    return 1;
                }

                // 调用解析器
                string parseResult = CallParseFile(filename, debug);
                // 只在调试模式或解析失败时显示解析结果
                if (debug || parseResult.Contains("错误"))
                {
                    LvlTerminal.Term.PrintLine(parseResult);
                }

                // 调用解释器
                string executeResult = CallExecuteFile(filename, debug);
                // 只在调试模式或执行失败时显示执行结果
                if (debug || executeResult.Contains("错误"))
                {
                    LvlTerminal.Term.PrintLine(executeResult);
                }

                return 0;
            }
            catch (Exception ex)
            {
                LvlTerminal.Term.ErrorLine($"程序异常: {ex.Message}");
                return 1;
            }
            finally
            {
                UnloadDlls();
            }
        }

        // 加载 DLL
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr LoadLibrary(string lpFileName);

        // 卸载 DLL
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool FreeLibrary(IntPtr hModule);

        // 获取函数地址
        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr GetProcAddress(IntPtr hModule, string lpProcName);

        private static IntPtr LoadDll(string dllPath)
        {
            IntPtr hModule = LoadLibrary(dllPath);
            if (hModule == IntPtr.Zero)
            {
                int error = Marshal.GetLastWin32Error();
                throw new DllNotFoundException($"无法加载 DLL: {dllPath}, 错误码: {error}");
            }
            return hModule;
        }

        private static void UnloadDlls()
        {
            if (_parserDll != IntPtr.Zero)
            {
                FreeLibrary(_parserDll);
                _parserDll = IntPtr.Zero;
            }
            if (_interpreterDll != IntPtr.Zero)
            {
                FreeLibrary(_interpreterDll);
                _interpreterDll = IntPtr.Zero;
            }
        }

        // 调用 DLL 函数
        private static string CallParseFile(string filename, bool debug)
        {
            IntPtr proc = GetProcAddress(_parserDll, debug ? "ParseFileDebug" : "ParseFile");
            if (proc == IntPtr.Zero)
            {
                return "错误: 函数未找到";
            }

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
            {
                return "";
            }

            string str = LvlTerminal.Term.FromAnsi(result);

            // 释放字符串内存
            IntPtr freeProc = GetProcAddress(_parserDll, "FreeString");
            if (freeProc != IntPtr.Zero)
            {
                var freeFunc = Marshal.GetDelegateForFunctionPointer<FreeStringDelegate>(freeProc);
                freeFunc(result);
            }

            return str;
        }

        private static string CallExecuteFile(string filename, bool debug)
        {
            IntPtr proc = GetProcAddress(_interpreterDll, debug ? "ExecuteFileDebug" : "ExecuteFile");
            if (proc == IntPtr.Zero)
            {
                return "错误: 函数未找到";
            }

            IntPtr result;
            if (debug)
            {
                var func = Marshal.GetDelegateForFunctionPointer<ExecuteFileDebugDelegate>(proc);
                result = func(filename);
            }
            else
            {
                var func = Marshal.GetDelegateForFunctionPointer<ExecuteFileDelegate>(proc);
                result = func(filename);
            }

            if (result == IntPtr.Zero)
            {
                return "";
            }

            string str = LvlTerminal.Term.FromAnsi(result);

            // 释放字符串内存
            IntPtr freeProc = GetProcAddress(_interpreterDll, "FreeString");
            if (freeProc != IntPtr.Zero)
            {
                var freeFunc = Marshal.GetDelegateForFunctionPointer<FreeStringDelegate>(freeProc);
                freeFunc(result);
            }

            return str;
        }
    }
}
