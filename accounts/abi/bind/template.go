
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package bind

import "github.com/ethereum/go-ethereum/accounts/abi"

//tmpldata是填充绑定模板所需的数据结构。
type tmplData struct {
Package   string                   //要将生成的文件放入其中的包的名称
Contracts map[string]*tmplContract //要生成到此文件中的合同列表
}

//tmplcontract包含生成单个合同绑定所需的数据。
type tmplContract struct {
Type        string                 //主合同绑定的类型名称
InputABI    string                 //json abi用作从中生成绑定的输入
InputBin    string                 //可选的EVM字节码，用于从中重新部署代码
Constructor abi.Method             //部署参数化的契约构造函数
Calls       map[string]*tmplMethod //合同调用只读取状态数据
Transacts   map[string]*tmplMethod //写状态数据的合同调用
Events      map[string]*tmplEvent  //合同事件访问器
}

//tmplmethod是一个abi.method的包装，其中包含一些预处理的
//和缓存的数据字段。
type tmplMethod struct {
Original   abi.Method //ABI包解析的原始方法
Normalized abi.Method //解析方法的规范化版本（大写名称、非匿名参数/返回）
Structured bool       //返回值是否应累积到结构中
}

//tmplevent是围绕
type tmplEvent struct {
Original   abi.Event //ABI包解析的原始事件
Normalized abi.Event //解析字段的规范化版本
}

//tmplsource是语言到模板的映射，包含所有支持的
//程序包可以生成的编程语言。
var tmplSource = map[Lang]string{
	LangGo:   tmplSourceGo,
	LangJava: tmplSourceJava,
}

//tmplsourcego是用于生成合同绑定的go源模板
//基于。
const tmplSourceGo = `
//代码生成-不要编辑。
//此文件是生成的绑定，任何手动更改都将丢失。

package {{.Package}}

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

//引用导入以禁止错误（如果未使用）。
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

{{range $contract := .Contracts}}
//.type abi是用于生成绑定的输入abi。
	const {{.Type}}ABI = "{{.InputABI}}"

	{{if .InputBin}}
//.type bin是用于部署新合同的编译字节码。
		const {{.Type}}Bin = ` + "`" + `{{.InputBin}}` + "`" + `

//部署。类型部署新的以太坊契约，将类型的实例绑定到它。
		func Deploy{{.Type}}(auth *bind.TransactOpts, backend bind.ContractBackend {{range .Constructor.Inputs}}, {{.Name}} {{bindtype .Type}}{{end}}) (common.Address, *types.Transaction, *{{.Type}}, error) {
		  parsed, err := abi.JSON(strings.NewReader({{.Type}}ABI))
		  if err != nil {
		    return common.Address{}, nil, nil, err
		  }
		  address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex({{.Type}}Bin), backend {{range .Constructor.Inputs}}, {{.Name}}{{end}})
		  if err != nil {
		    return common.Address{}, nil, nil, err
		  }
		  return address, tx, &{{.Type}}{ {{.Type}}Caller: {{.Type}}Caller{contract: contract}, {{.Type}}Transactor: {{.Type}}Transactor{contract: contract}, {{.Type}}Filterer: {{.Type}}Filterer{contract: contract} }, nil
		}
	{{end}}

//.type是围绕以太坊合约自动生成的go绑定。
	type {{.Type}} struct {
{{.Type}}Caller     //对合同具有只读约束力
{{.Type}}Transactor //只写对合同有约束力
{{.Type}}Filterer   //合同事件的日志筛选程序
	}

//.类型调用者是围绕以太坊契约自动生成的只读Go绑定。
	type {{.Type}}Caller struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
	}

//.type Transactior是围绕以太坊合同自动生成的只写Go绑定。
	type {{.Type}}Transactor struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
	}

//.type filter是围绕以太坊合同事件自动生成的日志筛选go绑定。
	type {{.Type}}Filterer struct {
contract *bind.BoundContract //用于低级调用的通用协定包装器
	}

//.type session是围绕以太坊合约自动生成的go绑定，
//具有预设的调用和事务处理选项。
	type {{.Type}}Session struct {
Contract     *{{.Type}}        //为其设置会话的通用约定绑定
CallOpts     bind.CallOpts     //在整个会话中使用的调用选项
TransactOpts bind.TransactOpts //要在此会话中使用的事务验证选项
	}

//.类型Callersession是围绕以太坊契约自动生成的只读Go绑定，
//带预设通话选项。
	type {{.Type}}CallerSession struct {
Contract *{{.Type}}Caller //用于设置会话的通用协定调用方绑定
CallOpts bind.CallOpts    //在整个会话中使用的调用选项
	}

//.类型事务会话是围绕以太坊合同自动生成的只写即用绑定，
//具有预设的Transact选项。
	type {{.Type}}TransactorSession struct {
Contract     *{{.Type}}Transactor //用于设置会话的通用合同事务处理程序绑定
TransactOpts bind.TransactOpts    //要在此会话中使用的事务验证选项
	}

//.type raw是围绕以太坊合约自动生成的低级go绑定。
	type {{.Type}}Raw struct {
Contract *{{.Type}} //用于访问上的原始方法的通用合同绑定
	}

//.type Callerraw是围绕以太坊合约自动生成的低级只读Go绑定。
	type {{.Type}}CallerRaw struct {
Contract *{{.Type}}Caller //用于访问上的原始方法的通用只读协定绑定
	}

//.type transactorraw是围绕以太坊合同自动生成的低级只写即用绑定。
	type {{.Type}}TransactorRaw struct {
Contract *{{.Type}}Transactor //用于访问上的原始方法的通用只写协定绑定
	}

//new。type创建绑定到特定部署合同的.type的新实例。
	func New{{.Type}}(address common.Address, backend bind.ContractBackend) (*{{.Type}}, error) {
	  contract, err := bind{{.Type}}(address, backend, backend, backend)
	  if err != nil {
	    return nil, err
	  }
	  return &{{.Type}}{ {{.Type}}Caller: {{.Type}}Caller{contract: contract}, {{.Type}}Transactor: {{.Type}}Transactor{contract: contract}, {{.Type}}Filterer: {{.Type}}Filterer{contract: contract} }, nil
	}

//new.type调用者创建绑定到特定部署的协定的.type的新只读实例。
	func New{{.Type}}Caller(address common.Address, caller bind.ContractCaller) (*{{.Type}}Caller, error) {
	  contract, err := bind{{.Type}}(address, caller, nil, nil)
	  if err != nil {
	    return nil, err
	  }
	  return &{{.Type}}Caller{contract: contract}, nil
	}

//新建。类型事务处理程序创建绑定到特定部署合同的类型的新只写实例。
	func New{{.Type}}Transactor(address common.Address, transactor bind.ContractTransactor) (*{{.Type}}Transactor, error) {
	  contract, err := bind{{.Type}}(address, nil, transactor, nil)
	  if err != nil {
	    return nil, err
	  }
	  return &{{.Type}}Transactor{contract: contract}, nil
	}

//new。type filter创建绑定到特定部署合同的.type的新日志筛选器实例。
 	func New{{.Type}}Filterer(address common.Address, filterer bind.ContractFilterer) (*{{.Type}}Filterer, error) {
 	  contract, err := bind{{.Type}}(address, nil, nil, filterer)
 	  if err != nil {
 	    return nil, err
 	  }
 	  return &{{.Type}}Filterer{contract: contract}, nil
 	}

//bind。type将通用包装绑定到已部署的合同。
	func bind{{.Type}}(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	  parsed, err := abi.JSON(strings.NewReader({{.Type}}ABI))
	  if err != nil {
	    return nil, err
	  }
	  return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
	}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
	func (_{{$contract.Type}} *{{$contract.Type}}Raw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
		return _{{$contract.Type}}.Contract.{{$contract.Type}}Caller.contract.Call(opts, result, method, params...)
	}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
	func (_{{$contract.Type}} *{{$contract.Type}}Raw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
		return _{{$contract.Type}}.Contract.{{$contract.Type}}Transactor.contract.Transfer(opts)
	}

//Transact使用参数作为输入值调用（付费）Contract方法。
	func (_{{$contract.Type}} *{{$contract.Type}}Raw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
		return _{{$contract.Type}}.Contract.{{$contract.Type}}Transactor.contract.Transact(opts, method, params...)
	}

//调用调用（常量）contract方法，参数作为输入值，并且
//将输出设置为结果。结果类型可能是用于
//返回、匿名返回的接口切片和命名的结构
//返回。
	func (_{{$contract.Type}} *{{$contract.Type}}CallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
		return _{{$contract.Type}}.Contract.contract.Call(opts, result, method, params...)
	}

//转账启动普通交易以将资金转移到合同，调用
//它的默认方法（如果有）。
	func (_{{$contract.Type}} *{{$contract.Type}}TransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
		return _{{$contract.Type}}.Contract.contract.Transfer(opts)
	}

//Transact使用参数作为输入值调用（付费）Contract方法。
	func (_{{$contract.Type}} *{{$contract.Type}}TransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
		return _{{$contract.Type}}.Contract.contract.Transact(opts, method, params...)
	}

	{{range .Calls}}
//.normalized.name是绑定协定方法0x printf“%x”.original.id的免费数据检索调用。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}Caller) {{.Normalized.Name}}(opts *bind.CallOpts {{range .Normalized.Inputs}}, {{.Name}} {{bindtype .Type}} {{end}}) ({{if .Structured}}struct{ {{range .Normalized.Outputs}}{{.Name}} {{bindtype .Type}};{{end}} },{{else}}{{range .Normalized.Outputs}}{{bindtype .Type}},{{end}}{{end}} error) {
			{{if .Structured}}ret := new(struct{
				{{range .Normalized.Outputs}}{{.Name}} {{bindtype .Type}}
				{{end}}
			}){{else}}var (
				{{range $i, $_ := .Normalized.Outputs}}ret{{$i}} = new({{bindtype .Type}})
				{{end}}
			){{end}}
			out := {{if .Structured}}ret{{else}}{{if eq (len .Normalized.Outputs) 1}}ret0{{else}}&[]interface{}{
				{{range $i, $_ := .Normalized.Outputs}}ret{{$i}},
				{{end}}
			}{{end}}{{end}}
			err := _{{$contract.Type}}.contract.Call(opts, out, "{{.Original.Name}}" {{range .Normalized.Inputs}}, {{.Name}}{{end}})
			return {{if .Structured}}*ret,{{else}}{{range $i, $_ := .Normalized.Outputs}}*ret{{$i}},{{end}}{{end}} err
		}

//.normalized.name是绑定协定方法0x printf“%x”.original.id的免费数据检索调用。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}Session) {{.Normalized.Name}}({{range $i, $_ := .Normalized.Inputs}}{{if ne $i 0}},{{end}} {{.Name}} {{bindtype .Type}} {{end}}) ({{if .Structured}}struct{ {{range .Normalized.Outputs}}{{.Name}} {{bindtype .Type}};{{end}} }, {{else}} {{range .Normalized.Outputs}}{{bindtype .Type}},{{end}} {{end}} error) {
		  return _{{$contract.Type}}.Contract.{{.Normalized.Name}}(&_{{$contract.Type}}.CallOpts {{range .Normalized.Inputs}}, {{.Name}}{{end}})
		}

//.normalized.name是绑定协定方法0x printf“%x”.original.id的免费数据检索调用。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}CallerSession) {{.Normalized.Name}}({{range $i, $_ := .Normalized.Inputs}}{{if ne $i 0}},{{end}} {{.Name}} {{bindtype .Type}} {{end}}) ({{if .Structured}}struct{ {{range .Normalized.Outputs}}{{.Name}} {{bindtype .Type}};{{end}} }, {{else}} {{range .Normalized.Outputs}}{{bindtype .Type}},{{end}} {{end}} error) {
		  return _{{$contract.Type}}.Contract.{{.Normalized.Name}}(&_{{$contract.Type}}.CallOpts {{range .Normalized.Inputs}}, {{.Name}}{{end}})
		}
	{{end}}

	{{range .Transacts}}
//.normalized.name是一个付费的转换程序事务，绑定合同方法0x printf“%x”.original.id。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}Transactor) {{.Normalized.Name}}(opts *bind.TransactOpts {{range .Normalized.Inputs}}, {{.Name}} {{bindtype .Type}} {{end}}) (*types.Transaction, error) {
			return _{{$contract.Type}}.contract.Transact(opts, "{{.Original.Name}}" {{range .Normalized.Inputs}}, {{.Name}}{{end}})
		}

//.normalized.name是一个付费的转换程序事务，绑定合同方法0x printf“%x”.original.id。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}Session) {{.Normalized.Name}}({{range $i, $_ := .Normalized.Inputs}}{{if ne $i 0}},{{end}} {{.Name}} {{bindtype .Type}} {{end}}) (*types.Transaction, error) {
		  return _{{$contract.Type}}.Contract.{{.Normalized.Name}}(&_{{$contract.Type}}.TransactOpts {{range $i, $_ := .Normalized.Inputs}}, {{.Name}}{{end}})
		}

//.normalized.name是一个付费的转换程序事务，绑定合同方法0x printf“%x”.original.id。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}TransactorSession) {{.Normalized.Name}}({{range $i, $_ := .Normalized.Inputs}}{{if ne $i 0}},{{end}} {{.Name}} {{bindtype .Type}} {{end}}) (*types.Transaction, error) {
		  return _{{$contract.Type}}.Contract.{{.Normalized.Name}}(&_{{$contract.Type}}.TransactOpts {{range $i, $_ := .Normalized.Inputs}}, {{.Name}}{{end}})
		}
	{{end}}

	{{range .Events}}
//$contract.type.normalized.name迭代器从filter.normalized.name返回，用于迭代.normalized.name contract.type合同引发的原始日志和未打包数据。
		type {{$contract.Type}}{{.Normalized.Name}}Iterator struct {
Event *{{$contract.Type}}{{.Normalized.Name}} //包含合同细节和原始日志的事件

contract *bind.BoundContract //用于解包事件数据的通用合同
event    string              //用于解包事件数据的事件名称

logs chan types.Log        //日志通道接收找到的合同事件
sub  ethereum.Subscription //错误、完成和终止订阅
done bool                  //订阅是否完成传递日志
fail error                 //停止迭代时出错
		}
//next将迭代器前进到后续事件，返回是否存在
//是否找到更多事件。在检索或分析错误的情况下，false是
//返回错误（），可以查询错误（）的确切错误。
		func (it *{{$contract.Type}}{{.Normalized.Name}}Iterator) Next() bool {
//如果迭代器失败，请停止迭代
			if (it.fail != nil) {
				return false
			}
//如果迭代器已完成，则直接传递可用的
			if (it.done) {
				select {
				case log := <-it.logs:
					it.Event = new({{$contract.Type}}{{.Normalized.Name}})
					if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
						it.fail = err
						return false
					}
					it.Event.Raw = log
					return true

				default:
					return false
				}
			}
//迭代器仍在进行中，请等待数据或错误事件
			select {
			case log := <-it.logs:
				it.Event = new({{$contract.Type}}{{.Normalized.Name}})
				if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
					it.fail = err
					return false
				}
				it.Event.Raw = log
				return true

			case err := <-it.sub.Err():
				it.done = true
				it.fail = err
				return it.Next()
			}
		}
//错误返回筛选期间发生的任何检索或分析错误。
		func (it *{{$contract.Type}}{{.Normalized.Name}}Iterator) Error() error {
			return it.fail
		}
//关闭终止迭代过程，释放任何挂起的基础
//资源。
		func (it *{{$contract.Type}}{{.Normalized.Name}}Iterator) Close() error {
			it.sub.Unsubscribe()
			return nil
		}

//$contract.type.normalized.name表示.normalized.name由$contract.type contract引发的事件。
		type {{$contract.Type}}{{.Normalized.Name}} struct { {{range .Normalized.Inputs}}
			{{capitalise .Name}} {{if .Indexed}}{{bindtopictype .Type}}{{else}}{{bindtype .Type}}{{end}}; {{end}}
Raw types.Log //区块链特定的上下文信息
		}

//filter.normalized.name是绑定协定事件0x printf“%x”.original.id的自由日志检索操作。
//
//坚固性：.original.string
 		func (_{{$contract.Type}} *{{$contract.Type}}Filterer) Filter{{.Normalized.Name}}(opts *bind.FilterOpts{{range .Normalized.Inputs}}{{if .Indexed}}, {{.Name}} []{{bindtype .Type}}{{end}}{{end}}) (*{{$contract.Type}}{{.Normalized.Name}}Iterator, error) {
			{{range .Normalized.Inputs}}
			{{if .Indexed}}var {{.Name}}Rule []interface{}
			for _, {{.Name}}Item := range {{.Name}} {
				{{.Name}}Rule = append({{.Name}}Rule, {{.Name}}Item)
			}{{end}}{{end}}

			logs, sub, err := _{{$contract.Type}}.contract.FilterLogs(opts, "{{.Original.Name}}"{{range .Normalized.Inputs}}{{if .Indexed}}, {{.Name}}Rule{{end}}{{end}})
			if err != nil {
				return nil, err
			}
			return &{{$contract.Type}}{{.Normalized.Name}}Iterator{contract: _{{$contract.Type}}.contract, event: "{{.Original.Name}}", logs: logs, sub: sub}, nil
 		}

//watch.normalized.name是绑定合同事件0x printf“%x”.original.id的自由日志订阅操作。
//
//坚固性：.original.string
		func (_{{$contract.Type}} *{{$contract.Type}}Filterer) Watch{{.Normalized.Name}}(opts *bind.WatchOpts, sink chan<- *{{$contract.Type}}{{.Normalized.Name}}{{range .Normalized.Inputs}}{{if .Indexed}}, {{.Name}} []{{bindtype .Type}}{{end}}{{end}}) (event.Subscription, error) {
			{{range .Normalized.Inputs}}
			{{if .Indexed}}var {{.Name}}Rule []interface{}
			for _, {{.Name}}Item := range {{.Name}} {
				{{.Name}}Rule = append({{.Name}}Rule, {{.Name}}Item)
			}{{end}}{{end}}

			logs, sub, err := _{{$contract.Type}}.contract.WatchLogs(opts, "{{.Original.Name}}"{{range .Normalized.Inputs}}{{if .Indexed}}, {{.Name}}Rule{{end}}{{end}})
			if err != nil {
				return nil, err
			}
			return event.NewSubscription(func(quit <-chan struct{}) error {
				defer sub.Unsubscribe()
				for {
					select {
					case log := <-logs:
//新日志到达，分析事件并转发给用户
						event := new({{$contract.Type}}{{.Normalized.Name}})
						if err := _{{$contract.Type}}.contract.UnpackLog(event, "{{.Original.Name}}", log); err != nil {
							return err
						}
						event.Raw = log

						select {
						case sink <- event:
						case err := <-sub.Err():
							return err
						case <-quit:
							return nil
						}
					case err := <-sub.Err():
						return err
					case <-quit:
						return nil
					}
				}
			}), nil
		}
 	{{end}}
{{end}}
`

//TMPLSURCEJAVA是用于生成合同绑定的Java源模板
//基于。
const tmplSourceJava = `
//这个文件是一个自动生成的Java绑定。不要修改为任何
//下一代人可能会失去变革！

package {{.Package}};

import org.ethereum.geth.*;
import org.ethereum.geth.internal.*;

{{range $contract := .Contracts}}
	public class {{.Type}} {
//abi是用于从中生成绑定的输入abi。
		public final static String ABI = "{{.InputABI}}";

		{{if .InputBin}}
//字节码是用于部署新合同的已编译字节码。
			public final static byte[] BYTECODE = "{{.InputBin}}".getBytes();

//Deploy部署一个新的以太坊契约，将.type的实例绑定到它。
			public static {{.Type}} deploy(TransactOpts auth, EthereumClient client{{range .Constructor.Inputs}}, {{bindtype .Type}} {{.Name}}{{end}}) throws Exception {
				Interfaces args = Geth.newInterfaces({{(len .Constructor.Inputs)}});
				{{range $index, $element := .Constructor.Inputs}}
				  args.set({{$index}}, Geth.newInterface()); args.get({{$index}}).set{{namedtype (bindtype .Type) .Type}}({{.Name}});
				{{end}}
				return new {{.Type}}(Geth.deployContract(auth, ABI, BYTECODE, client, args));
			}

//合同部署使用的内部构造函数。
			private {{.Type}}(BoundContract deployment) {
				this.Address  = deployment.getAddress();
				this.Deployer = deployment.getDeployer();
				this.Contract = deployment;
			}
		{{end}}

//本合同所在的以太坊地址。
		public final Address Address;

//部署此合同的以太坊事务（如果知道！）.
		public final Transaction Deployer;

//绑定到区块链地址的合同实例。
		private final BoundContract Contract;

//创建绑定到特定部署合同的.Type的新实例。
		public {{.Type}}(Address address, EthereumClient client) throws Exception {
			this(Geth.bindContract(address, ABI, client));
		}

		{{range .Calls}}
			{{if gt (len .Normalized.Outputs) 1}}
//capitale.normalized.name results是对.normalized.name的调用的输出。
			public class {{capitalise .Normalized.Name}}Results {
				{{range $index, $item := .Normalized.Outputs}}public {{bindtype .Type}} {{if ne .Name ""}}{{.Name}}{{else}}Return{{$index}}{{end}};
				{{end}}
			}
			{{end}}

//.normalized.name是绑定协定方法0x printf“%x”.original.id的免费数据检索调用。
//
//坚固性：.original.string
			public {{if gt (len .Normalized.Outputs) 1}}{{capitalise .Normalized.Name}}Results{{else}}{{range .Normalized.Outputs}}{{bindtype .Type}}{{end}}{{end}} {{.Normalized.Name}}(CallOpts opts{{range .Normalized.Inputs}}, {{bindtype .Type}} {{.Name}}{{end}}) throws Exception {
				Interfaces args = Geth.newInterfaces({{(len .Normalized.Inputs)}});
				{{range $index, $item := .Normalized.Inputs}}args.set({{$index}}, Geth.newInterface()); args.get({{$index}}).set{{namedtype (bindtype .Type) .Type}}({{.Name}});
				{{end}}

				Interfaces results = Geth.newInterfaces({{(len .Normalized.Outputs)}});
				{{range $index, $item := .Normalized.Outputs}}Interface result{{$index}} = Geth.newInterface(); result{{$index}}.setDefault{{namedtype (bindtype .Type) .Type}}(); results.set({{$index}}, result{{$index}});
				{{end}}

				if (opts == null) {
					opts = Geth.newCallOpts();
				}
				this.Contract.call(opts, results, "{{.Original.Name}}", args);
				{{if gt (len .Normalized.Outputs) 1}}
					{{capitalise .Normalized.Name}}Results result = new {{capitalise .Normalized.Name}}Results();
					{{range $index, $item := .Normalized.Outputs}}result.{{if ne .Name ""}}{{.Name}}{{else}}Return{{$index}}{{end}} = results.get({{$index}}).get{{namedtype (bindtype .Type) .Type}}();
					{{end}}
					return result;
				{{else}}{{range .Normalized.Outputs}}return results.get(0).get{{namedtype (bindtype .Type) .Type}}();{{end}}
				{{end}}
			}
		{{end}}

		{{range .Transacts}}
//.normalized.name是一个付费的转换程序事务，绑定合同方法0x printf“%x”.original.id。
//
//坚固性：.original.string
			public Transaction {{.Normalized.Name}}(TransactOpts opts{{range .Normalized.Inputs}}, {{bindtype .Type}} {{.Name}}{{end}}) throws Exception {
				Interfaces args = Geth.newInterfaces({{(len .Normalized.Inputs)}});
				{{range $index, $item := .Normalized.Inputs}}args.set({{$index}}, Geth.newInterface()); args.get({{$index}}).set{{namedtype (bindtype .Type) .Type}}({{.Name}});
				{{end}}

				return this.Contract.transact(opts, "{{.Original.Name}}"	, args);
			}
		{{end}}
	}
{{end}}
`
